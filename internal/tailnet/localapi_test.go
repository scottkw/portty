package tailnet

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tailscale.com/client/local"
)

// TestLocalAPI_OneClientKeepsOneConnection is the 23-09 goroutine growth in
// miniature, against a fake LocalAPI (Dial and OmitAuth: nothing real is
// contacted). A local.Client per call opens, and strands, one keep-alive
// connection per call; one shared client reuses a single connection.
func TestLocalAPI_OneClientKeepsOneConnection(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"BackendState":"Running"}`))
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	ctx := context.Background()

	shared := local.Client{Dial: dial, OmitAuth: true}
	for range 20 {
		if _, err := shared.StatusWithoutPeers(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 1 {
		t.Fatalf("one shared client opened %d connections for 20 calls, want 1", n)
	}

	conns.Store(0)
	for range 20 {
		lc := local.Client{Dial: dial, OmitAuth: true}
		if _, err := lc.StatusWithoutPeers(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 20 {
		t.Fatalf("a client per call opened %d connections for 20 calls, want 20 (the premise of LocalAPI)", n)
	}
}

// TestLocalAPI_NoClientPerCall: no shipped Go file builds a local.Client inside
// a function; LocalAPI is the one client. Build-tagged files (VM fakes, never
// in a release) are skipped.
func TestLocalAPI_NoClientPerCall(t *testing.T) {
	root := filepath.Join("..", "..")
	isClient := func(e ast.Expr) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := s.X.(*ast.Ident)
		return ok && x.Name == "local" && s.Sel.Name == "Client"
	}
	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", ".planning", "node_modules", "frontend", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(src), "\n//go:build ") || strings.HasPrefix(string(src), "//go:build ") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return err
		}
		checked++
		ast.Inspect(f, func(n ast.Node) bool {
			var body *ast.BlockStmt
			switch fn := n.(type) {
			case *ast.FuncDecl:
				body = fn.Body
			case *ast.FuncLit:
				body = fn.Body
			default:
				return true
			}
			ast.Inspect(body, func(n ast.Node) bool {
				hit := false
				switch x := n.(type) {
				case *ast.ValueSpec:
					hit = x.Type != nil && isClient(x.Type)
				case *ast.CompositeLit:
					hit = x.Type != nil && isClient(x.Type)
				case *ast.CallExpr:
					id, ok := x.Fun.(*ast.Ident)
					hit = ok && id.Name == "new" && len(x.Args) == 1 && isClient(x.Args[0])
				}
				if hit {
					t.Errorf("%s: a local.Client built per call strands a tailscaled connection each call; use tailnet.LocalAPI", fset.Position(n.Pos()))
				}
				return true
			})
			return false
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 100 {
		t.Fatalf("checked %d Go files under %s, want the whole module (the check would be vacuous)", checked, root)
	}
}
