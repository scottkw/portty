package desktop_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/scottkw/portty/internal/desktop"
)

func TestDesktopConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := desktop.Config{AlwaysAvailable: true, GrantID: "00112233445566778899aabbccddeeff", IAT: 1_700_000_000}
	if err := desktop.SaveConfig(dir, want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dir, desktop.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", fi.Mode().Perm())
	}
	got, err := desktop.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want.SchemaVersion = desktop.ConfigSchemaVersion
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("config dir holds %d entries after a save, want only desktop.json (no temp file left)", len(entries))
	}
}

func TestDesktopConfig_MissingIsOff(t *testing.T) {
	got, err := desktop.LoadConfig(t.TempDir())
	if err != nil || got != (desktop.Config{}) {
		t.Fatalf("LoadConfig(missing) = %+v, %v; want the zero Config and nil", got, err)
	}
}

func TestDesktopConfig_NewGrantID(t *testing.T) {
	a, err := desktop.NewGrantID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := desktop.NewGrantID()
	if len(a) != 32 || a == b {
		t.Errorf("grant ids %q and %q: want two distinct 32-char hex strings", a, b)
	}
}

// TestDesktopConfig_Unsafe: every way desktop.json can be wrong returns an error
// wrapping ErrConfigUnsafe (or the decode error), never a silent default.
func TestDesktopConfig_Unsafe(t *testing.T) {
	good := `{"schemaVersion":1,"alwaysAvailable":true,"grantID":"abc","iat":1}`
	write := func(t *testing.T, dir, body string, mode os.FileMode) string {
		t.Helper()
		p := filepath.Join(dir, desktop.ConfigFile)
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil { // the umask must not decide the mode under test
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"symlink", func(t *testing.T, dir string) {
			target := write(t, t.TempDir(), good, 0o600)
			if err := os.Symlink(target, filepath.Join(dir, desktop.ConfigFile)); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory", func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, desktop.ConfigFile), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode 0644", func(t *testing.T, dir string) { write(t, dir, good, 0o644) }},
		{"mode 0640", func(t *testing.T, dir string) { write(t, dir, good, 0o640) }},
		{"over 4 KiB", func(t *testing.T, dir string) {
			pad := make([]byte, 5000)
			for i := range pad {
				pad[i] = ' '
			}
			write(t, dir, good+string(pad), 0o600)
		}},
		{"trailing data", func(t *testing.T, dir string) { write(t, dir, good+`{}`, 0o600) }},
		{"schemaVersion 2", func(t *testing.T, dir string) {
			write(t, dir, `{"schemaVersion":2,"alwaysAvailable":true,"grantID":"abc","iat":1}`, 0o600)
		}},
		{"schemaVersion missing", func(t *testing.T, dir string) {
			write(t, dir, `{"alwaysAvailable":false,"grantID":"","iat":0}`, 0o600)
		}},
		{"unknown field", func(t *testing.T, dir string) {
			write(t, dir, `{"schemaVersion":1,"alwaysAvailable":false,"grantID":"","iat":0,"extra":1}`, 0o600)
		}},
		{"invalid JSON", func(t *testing.T, dir string) { write(t, dir, `{"schemaVersion":`, 0o600) }},
		{"empty file", func(t *testing.T, dir string) { write(t, dir, ``, 0o600) }},
		{"on without a grant", func(t *testing.T, dir string) {
			write(t, dir, `{"schemaVersion":1,"alwaysAvailable":true,"grantID":"","iat":0}`, 0o600)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c.setup(t, dir)
			got, err := desktop.LoadConfig(dir)
			if err == nil {
				t.Fatalf("LoadConfig = %+v, nil; want an error", got)
			}
			if got != (desktop.Config{}) {
				t.Errorf("an error came with a non-zero Config %+v", got)
			}
		})
	}

	t.Run("the good body loads", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, good, 0o600)
		got, err := desktop.LoadConfig(dir)
		if err != nil || !got.AlwaysAvailable || got.GrantID != "abc" {
			t.Fatalf("LoadConfig = %+v, %v; want the saved values (the cases above fail for their own reason)", got, err)
		}
	})

	t.Run("errors from a bad file name the sentinel", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, good, 0o644)
		if _, err := desktop.LoadConfig(dir); !errors.Is(err, desktop.ErrConfigUnsafe) {
			t.Errorf("err = %v, want errors.Is ErrConfigUnsafe", err)
		}
	})
}
