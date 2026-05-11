# gopub

Go library for reading EPUB 2.0 and 3.0 files.

## Install

```
go get github.com/LapisApple/go-epub
```

```go
import "github.com/LapisApple/go-epub/gopub"
```

## Quick Start

**From a file path:**

```go
r, err := gopub.OpenReader("book.epub")
if err != nil {
    log.Fatal(err)
}
defer r.Close()

rf := r.Container.DefaultRendition()
fmt.Println(rf.Metadata.MainTitle().Name)
fmt.Println(rf.Metadata.Creator[0].Name)
```

**From an `io.ReaderAt` (e.g. an `*os.File`):**

```go
f, _ := os.Open("book.epub")
defer f.Close()
fi, _ := f.Stat()

r, err := gopub.NewReader(f, fi.Size())
```

**Iterate reading order:**

```go
for _, item := range rf.Spine.Itemrefs {
    if !item.IsLinear() {
        continue
    }
    rc, _ := item.Open()
    data, _ := io.ReadAll(rc)
    rc.Close()
    _ = data
}
```

**Navigation:**

```go
// EPUB 3.0
if toc := rf.TOCNav(); toc != nil {
    for _, entry := range toc.Items {
        fmt.Println(entry.Link.Text, entry.Link.Href)
    }
}

// EPUB 2.0 fallback
for _, point := range rf.NCX.NavPoints {
    fmt.Println(point.NavLabel.Text, point.Content.Src)
}
```

**Cover image:**

```go
cover, err := r.GetCover()
if err != nil {
    log.Fatal(err)
}
rc, _ := cover.Open()
defer rc.Close()
```

**Guard against ZIP bombs:**

```go
opts := gopub.ReaderOptions{MaxFileSize: 50 << 20} // 50 MB
r, err := gopub.OpenReader("untrusted.epub", opts)
```

## Features

- EPUB 2.0 and 3.0
- Rich metadata: refinements, file-as, role, title-type, series, series index, modified, writing mode
- EPUB 3.0 NavDoc + EPUB 2.0 NCX navigation
- Cover extraction — unwraps SVG and XHTML wrappers, falls back to EPUB 2.0 guide
- Malformed XML tolerance: invalid `&`, non-ASCII tag names, UTF-8 BOM
- `MaxFileSize` option to reject oversized files

## API

| Entry point | Returns | Description |
|---|---|---|
| `OpenReader(path, ...opts)` | `*ReadCloser, error` | Open EPUB from disk |
| `NewReader(ra, size, ...opts)` | `*Reader, error` | Open from `io.ReaderAt` |

| Type | Key fields / methods |
|---|---|
| `Container` | `Rootfiles`, `DefaultRendition()` |
| `Rootfile` | `Metadata`, `Manifest`, `Spine`, `NCX`, `NavDoc`, `TOCNav()`, `ItemName(href)` |
| `Manifest` | `Items`, `Stylesheets()`, `Images()`, `Fonts()` |
| `ManifestItem` | `ID`, `HREF`, `MediaType`, `Open()` |
| `Spine` | `Itemrefs` (`SpineItem` resolves to `*ManifestItem`) |
| `Metadata` | `MainTitle()`, `Creator`, `Language`, `Identifier`, `Series`, … |

## Legal

MIT License — Copyright © 2026 LapisApple.

Forked from [`github.com/taylorskalyo/goreader`](https://github.com/taylorskalyo/goreader) (epub package).  
Feature inspiration from the Rust crate [`epub`](https://crates.io/crates/epub).
