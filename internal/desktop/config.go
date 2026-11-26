package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ConfigFile is the always-available setting's own file in the config dir. It
// is not part of settings.json: an older daemon drops fields it does not know
// when it saves settings, which would erase the grant and break saved links.
const ConfigFile = "desktop.json"

// ConfigSchemaVersion is the only schema this build reads or writes.
const ConfigSchemaVersion = 1

// configMaxSize bounds what LoadConfig reads; a real file is under 200 bytes.
const configMaxSize = 4 << 10

// ErrConfigUnsafe wraps every reason LoadConfig refuses a desktop.json that
// exists: not a regular file, wrong mode, too big, unknown schema, bad JSON.
var ErrConfigUnsafe = errors.New("desktop: desktop.json is not usable")

// Config is the persisted desktop setting. GrantID and IAT are written once at
// turn-on, so the link the HMAC signs over them is the same after every
// restart. Turning the desktop off clears them.
type Config struct {
	SchemaVersion   int    `json:"schemaVersion"`
	AlwaysAvailable bool   `json:"alwaysAvailable"`
	GrantID         string `json:"grantID"`
	IAT             int64  `json:"iat"`
}

func unsafeConfig(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConfigUnsafe, fmt.Sprintf(format, args...))
}

// LoadConfig reads desktop.json from dir. A missing file is the zero Config
// (always-available off) with no error. A file that exists but is not exactly
// what SaveConfig writes returns an error and is never replaced here: the
// caller keeps the desktop off and leaves the file for the user.
func LoadConfig(dir string) (Config, error) {
	p := filepath.Join(dir, ConfigFile)
	fi, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	if !fi.Mode().IsRegular() {
		return Config{}, unsafeConfig("not a regular file")
	}
	if fi.Mode().Perm() != 0o600 {
		return Config{}, unsafeConfig("mode is %o, want 600", fi.Mode().Perm())
	}
	if fi.Size() > configMaxSize {
		return Config{}, unsafeConfig("larger than %d bytes", configMaxSize)
	}
	f, err := os.Open(p)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = f.Close() }()
	// A swap between the Lstat and the Open (a symlink put in place) fails here.
	if fi2, err := f.Stat(); err != nil || !os.SameFile(fi, fi2) {
		return Config{}, unsafeConfig("changed while it was read")
	}
	dec := json.NewDecoder(io.LimitReader(f, configMaxSize+1))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrConfigUnsafe, err)
	}
	if c.SchemaVersion != ConfigSchemaVersion {
		return Config{}, unsafeConfig("schemaVersion %d, want %d", c.SchemaVersion, ConfigSchemaVersion)
	}
	if c.AlwaysAvailable && c.GrantID == "" {
		return Config{}, unsafeConfig("alwaysAvailable without a grantID")
	}
	return c, nil
}

// SaveConfig writes c as desktop.json in dir: a 0600 temp file in the same
// directory, synced, then renamed over the old one, so a crash leaves either
// the old file or the new, never half of one. The schema version is always the
// current one.
func SaveConfig(dir string, c Config) error {
	c.SchemaVersion = ConfigSchemaVersion
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ConfigFile+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, ConfigFile))
}

// NewGrantID returns 128 random bits as hex, the id the link's token names.
func NewGrantID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
