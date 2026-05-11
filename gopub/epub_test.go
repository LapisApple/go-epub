package gopub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const expFormat = "Expected: %v, but got: %v\n"

type containerTest struct {
	*testing.T
	c Container
}

type spineItem struct {
	idx   int
	idref string
}

type manifestItem struct {
	idx  int
	id   string
	href string
}

type epubExpected struct {
	title         string
	creator       string
	spineItems    []spineItem
	manifestItems []manifestItem
}

var epubExpectations = map[string]epubExpected{
	"alice.epub": {
		title:   "Alice's Adventures in Wonderland / Illustrated by Arthur Rackham. With a Proem by Austin Dobson",
		creator: "Lewis Carroll",
		spineItems: []spineItem{
			{0, "coverpage-wrapper"},
			{1, "item41"},
		},
		manifestItems: []manifestItem{
			{40, "item41", "@public@vhost@g@gutenberg@html@files@28885@28885-h@28885-h-0.htm.html"},
			{0, "item1", "@public@vhost@g@gutenberg@html@files@28885@28885-h@images@cover.jpg"},
		},
	},
}

func listTestEpubs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("_test_files/")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".epub") {
			paths = append(paths, "_test_files/"+e.Name())
		}
	}
	return paths
}

func TestOpenReader(t *testing.T) {
	for _, epubPath := range listTestEpubs(t) {
		name := filepath.Base(epubPath)
		t.Run(name, func(t *testing.T) {
			r, err := OpenReader(epubPath)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()

			if exp, ok := epubExpectations[name]; ok {
				ct := containerTest{t, r.Container}
				ct.TestContainer(exp)
			} else if len(r.Container.Rootfiles) == 0 {
				t.Error("no rootfiles")
			}
		})
	}
}

func TestNewReader(t *testing.T) {
	for _, epubPath := range listTestEpubs(t) {
		name := filepath.Base(epubPath)
		t.Run(name, func(t *testing.T) {
			rc, err := os.Open(epubPath)
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()

			fi, err := rc.Stat()
			if err != nil {
				t.Fatal(err)
			}

			r, err := NewReader(rc, fi.Size())
			if err != nil {
				t.Fatal(err)
			}

			if exp, ok := epubExpectations[name]; ok {
				ct := containerTest{t, r.Container}
				ct.TestContainer(exp)
			} else if len(r.Container.Rootfiles) == 0 {
				t.Error("no rootfiles")
			}
		})
	}
}

func (ct *containerTest) TestContainer(exp epubExpected) {
	ct.Run("Container", func(t *testing.T) {
		tt := containerTest{t, ct.c}
		tt.TestMetadata(exp.title, exp.creator)
		tt.TestSpine(exp.spineItems)
		tt.TestManifest(exp.manifestItems)
	})
}

func TestGetCover(t *testing.T) {
	for _, epubPath := range listTestEpubs(t) {
		name := filepath.Base(epubPath)
		t.Run(name, func(t *testing.T) {
			r, err := OpenReader(epubPath)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()

			cover, err := r.GetCover()
			if err == ErrMissingCoverId {
				t.Skip("no cover id declared")
			}
			if err != nil {
				t.Fatalf("GetCover: %v", err)
			}
			if cover.MediaType == "image/svg+xml" {
				t.Errorf("GetCover returned SVG wrapper (href=%s), expected actual image", cover.HREF)
			}
			rc, err := cover.Open()
			if err != nil {
				t.Fatalf("cover.Open: %v", err)
			}
			rc.Close()
		})
	}
}

func (ct *containerTest) TestMetadata(expTitle, expCreator string) {
	meta := ct.c.Rootfiles[0].Metadata

	if meta.Title.Name != expTitle {
		ct.Errorf(expFormat, expTitle, meta.Title)
	}

	if meta.Creator[0].Name != expCreator {
		ct.Errorf(expFormat, expCreator, meta.Creator)
	}
}

func (ct *containerTest) TestSpine(cases []spineItem) {
	spine := ct.c.Rootfiles[0].Spine
	for _, tc := range cases {
		ct.Run("Item", func(t *testing.T) {
			itemref := spine.Itemrefs[tc.idx]
			if itemref.IDREF != tc.idref {
				t.Errorf(expFormat, tc.idref, itemref.IDREF)
			}

			if itemref.ManifestItem == nil {
				t.Errorf(expFormat, "not nil", "nil")
			} else if itemref.ManifestItem.ID != tc.idref {
				t.Errorf(expFormat, tc.idref, itemref.ManifestItem.ID)
			}
		})
	}
}

func (ct *containerTest) TestManifest(cases []manifestItem) {
	manifest := ct.c.Rootfiles[0].Manifest
	for _, tc := range cases {
		ct.Run("Item", func(t *testing.T) {
			item := manifest.Items[tc.idx]

			if item.ID != tc.id {
				t.Errorf(expFormat, tc.id, item.ID)
			}

			if item.HREF != tc.href {
				t.Errorf(expFormat, tc.href, item.HREF)
			}
		})
	}
}
