//go:build windows

package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	advapi32              = windows.NewLazySystemDLL("advapi32.dll")
	userenv               = windows.NewLazySystemDLL("userenv.dll")
	procLogonUser         = advapi32.NewProc("LogonUserW")
	procImpersonate       = advapi32.NewProc("ImpersonateLoggedOnUser")
	procRevertToSelf      = advapi32.NewProc("RevertToSelf")
	procLoadUserProfile   = userenv.NewProc("LoadUserProfileW")
	procUnloadUserProfile = userenv.NewProc("UnloadUserProfile")
)

type profileInfo struct {
	Size        uint32
	Flags       uint32
	UserName    *uint16
	ProfilePath *uint16
	DefaultPath *uint16
	ServerName  *uint16
	PolicyPath  *uint16
	Profile     syscall.Handle
}

const usageText = `protect.exe - schreibt DPAPI-geschützte Werte in die Registry (HKCU) eines anderen Benutzers

Aufruf:
  protect.exe <Key-Pfad> name=wert [name=wert ...]
  protect.exe -h | --help | /?

Argumente:
  <Key-Pfad>    Pfad relativ zu HKCU des Zielbenutzers, ohne führenden Backslash
                z. B. Software\MeineAnwendung
  name=wert     Registry-Wertname und Klartext-Wert; beliebig viele möglich
                (vorhandene Werte gleichen Namens werden überschrieben)

Zugangsdaten des Zielbenutzers (Umgebungsvariablen):
  T_USER        Benutzername
  T_DOMAIN      Domäne, bei lokalen Konten "." oder der Rechnername
  T_PASS        Passwort (optional; wenn nicht gesetzt, wird es verdeckt abgefragt)

Voraussetzungen:
  - Als Administrator starten (nötig für LoadUserProfile)
  - Entschlüsseln kann den Wert danach nur der Zielbenutzer auf diesem Rechner

Beispiel:
  set T_USER=test
  set T_DOMAIN=.
  protect.exe "Software\MeineAnwendung" pass1=abc pass2=def pass3=ghi

Exitcodes:
  0 = OK, 1 = Fehler bei der Ausführung, 2 = falsche Argumente
`

func usage() { fmt.Print(usageText) }

// Liest eine Zeile von der Konsole ohne Echo.
func readPassword(prompt string) (string, error) {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return "", fmt.Errorf("stdin ist keine Konsole, bitte T_PASS setzen: %w", err)
	}
	if err := windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return "", err
	}
	defer windows.SetConsoleMode(h, mode)

	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr) // Zeilenumbruch, weil die Eingabe nicht angezeigt wurde
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func protectOne(secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("leerer Wert")
	}
	in := []byte(secret)
	inBlob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&inBlob, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	enc := make([]byte, out.Size)
	copy(enc, unsafe.Slice(out.Data, out.Size))
	return base64.StdEncoding.EncodeToString(enc), nil
}

// Schreibt beliebig viele Werte unter HKCU\<keyPath> des Zielbenutzers.
func protectAs(user, domain, pass, keyPath string, values map[string]string) error {
	u, _ := syscall.UTF16PtrFromString(user)
	d, _ := syscall.UTF16PtrFromString(domain)
	p, _ := syscall.UTF16PtrFromString(pass)

	var token windows.Token
	r, _, e := procLogonUser.Call(uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(p)), 2 /*LOGON32_LOGON_INTERACTIVE*/, 0, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return fmt.Errorf("LogonUser: %v", e)
	}
	defer token.Close()

	pi := profileInfo{UserName: u, Flags: 1 /*PI_NOUI*/}
	pi.Size = uint32(unsafe.Sizeof(pi))
	if r, _, e = procLoadUserProfile.Call(uintptr(token), uintptr(unsafe.Pointer(&pi))); r == 0 {
		return fmt.Errorf("LoadUserProfile (als Admin gestartet?): %v", e)
	}
	defer procUnloadUserProfile.Call(uintptr(token), uintptr(pi.Profile))

	// 1) alle Werte im Kontext des Zielbenutzers verschlüsseln
	encrypted, err := func() (map[string]string, error) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if r, _, e := procImpersonate.Call(uintptr(token)); r == 0 {
			return nil, fmt.Errorf("ImpersonateLoggedOnUser: %v", e)
		}
		defer procRevertToSelf.Call()

		res := make(map[string]string, len(values))
		for name, secret := range values {
			b64, err := protectOne(secret)
			if err != nil {
				return nil, fmt.Errorf("Wert '%s': %w", name, err)
			}
			res[name] = b64
		}
		return res, nil
	}()
	if err != nil {
		return err
	}

	// 2) Key einmal öffnen/anlegen, alle Werte schreiben
	k, _, err := registry.CreateKey(registry.Key(pi.Profile), keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("CreateKey: %w", err)
	}
	defer k.Close() // läuft vor UnloadUserProfile
	for name, b64 := range encrypted {
		if err := k.SetStringValue(name, b64); err != nil {
			return fmt.Errorf("SetStringValue '%s': %w", name, err)
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "-h", "-?", "--help", "/?", "/h":
		usage()
		return
	}
	if len(os.Args) < 3 {
		fmt.Println("Fehler: mindestens ein name=wert erforderlich.")
		fmt.Println("Hilfe: protect.exe -h")
		os.Exit(2)
	}

	user, domain, pass := os.Getenv("T_USER"), os.Getenv("T_DOMAIN"), os.Getenv("T_PASS")
	if user == "" {
		fmt.Println("Fehler: T_USER muss gesetzt sein.")
		fmt.Println("Hilfe: protect.exe -h")
		os.Exit(2)
	}
	if domain == "" {
		domain = "."
	}
	if pass == "" {
		var err error
		if pass, err = readPassword("Passwort für " + domain + "\\" + user + ": "); err != nil || pass == "" {
			fmt.Println("Fehler: kein Passwort eingegeben.", err)
			os.Exit(2)
		}
	}

	values := map[string]string{}
	for i, a := range os.Args[2:] {
		name, val, ok := strings.Cut(a, "=")
		if !ok || name == "" {
			// bewusst keine Ausgabe des Arguments, es könnte ein Passwort sein
			fmt.Printf("Ungültiges Argument Nr. %d (erwartet name=wert)\n", i+1)
			os.Exit(2)
		}
		values[name] = val
	}

	if err := protectAs(user, domain, pass, os.Args[1], values); err != nil {
		fmt.Println("Fehler:", err)
		os.Exit(1)
	}
	fmt.Println("OK:", len(values), "Wert(e) geschrieben")
}
