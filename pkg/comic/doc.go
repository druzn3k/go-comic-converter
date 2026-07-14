// Package comic provides a library-first API for converting comic archives
// (CBZ, CBR, directories, PDF) to e-reader formats (KEPUB, CBZ, HTML).
//
// For EPUB output, use pkg/epub which wraps this package.
//
// # Library usage
//
//	opts := comic.Options{
//	    Input:  "manga.cbz",
//	    Output: "manga.cbz",
//	    Image: epuboptions.Image{
//	        Quality:  85,
//	        GrayScale: true,
//	        Crop:    epuboptions.Crop{Enabled: true, Left: 1, Up: 1, Right: 1, Bottom: 3},
//	        View:    epuboptions.View{Width: 1072, Height: 1448},
//	        Resize:  true,
//	        Format:  "jpeg",
//	    },
//	    SortPathMode: 1,
//	    OutputFormat: "cbz",
//	}
//	if err := comic.New(opts).Convert(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//
// Custom output writers can be registered via output.Register.
// Custom source loaders can be registered via comic/source.New.
package comic
