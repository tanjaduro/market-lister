package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectFolders(t *testing.T) {
	tmp := t.TempDir()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(tmp, "alpha"), 0o755))
	must(os.Mkdir(filepath.Join(tmp, "beta"), 0o755))
	must(os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte("x"), 0o644))

	t.Run("no filter returns all dirs sorted", func(t *testing.T) {
		got, err := collectFolders(tmp, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2: %v", len(got), got)
		}
		if filepath.Base(got[0]) != "alpha" || filepath.Base(got[1]) != "beta" {
			t.Errorf("got %v, want [alpha, beta]", got)
		}
	})

	t.Run("filter selects matching subdir", func(t *testing.T) {
		got, err := collectFolders(tmp, "beta")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || filepath.Base(got[0]) != "beta" {
			t.Errorf("got %v, want [beta]", got)
		}
	})

	t.Run("filter to missing subdir returns empty", func(t *testing.T) {
		got, err := collectFolders(tmp, "gamma")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("nonexistent input dir returns error", func(t *testing.T) {
		if _, err := collectFolders(filepath.Join(tmp, "does-not-exist"), ""); err == nil {
			t.Error("expected error for missing input dir")
		}
	})
}

func TestOutputMDPath(t *testing.T) {
	cases := []struct {
		name       string
		cfg        Config
		folderPath string
		slug       string
		want       string
	}{
		{
			name:       "no OutputDir co-locates .md with photos",
			cfg:        Config{},
			folderPath: "/in/foo",
			slug:       "foo",
			want:       filepath.Join("/in/foo", "foo.md"),
		},
		{
			name:       "OutputDir set routes .md there",
			cfg:        Config{OutputDir: "/out"},
			folderPath: "/in/foo",
			slug:       "foo",
			want:       filepath.Join("/out", "foo.md"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := outputMDPath(tc.cfg, tc.folderPath, tc.slug); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProcessFolder_SkipsExistingOutput(t *testing.T) {
	tmp := t.TempDir()
	folderPath := filepath.Join(tmp, "myfolder")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "myfolder.md"), []byte("already here"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := processFolder(Config{}, nil, folderPath)
	if got != resultSkipped {
		t.Errorf("got %v, want resultSkipped", got)
	}
}

func TestProcessFolder_SkipsOnStatError(t *testing.T) {
	tmp := t.TempDir()
	folderPath := filepath.Join(tmp, "myfolder")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Force a non-NotExist Stat error: OutputDir points at a regular file, so
	// Stat of <file>/myfolder.md returns ENOTDIR, which is NOT ErrNotExist.
	fileAsOutputDir := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(fileAsOutputDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := processFolder(Config{OutputDir: fileAsOutputDir}, nil, folderPath)
	if got != resultSkipped {
		t.Errorf("got %v, want resultSkipped", got)
	}
}

func TestProcessFolder_SkipsOnEmptySlug(t *testing.T) {
	tmp := t.TempDir()
	// Folder name made entirely of non-ASCII chars slugifies to "".
	folderPath := filepath.Join(tmp, "中文")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := processFolder(Config{}, nil, folderPath)
	if got != resultSkipped {
		t.Errorf("got %v, want resultSkipped", got)
	}
}
