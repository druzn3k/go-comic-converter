// Package epuboptions for EPUB creation.
package epuboptions

type EPUBOptions struct {
	// Output
	Input  string `yaml:"-" json:"input"`
	Output string `yaml:"-" json:"output"`
	Author string `yaml:"-" json:"author"`
	Title  string `yaml:"-" json:"title"`

	// Metadata for ComicInfo.xml
	Series  string `yaml:"-" json:"series"`
	Number  string `yaml:"-" json:"number"`
	Summary string `yaml:"-" json:"summary"`
	Genre   string `yaml:"-" json:"genre"`
	Manga   bool   `yaml:"-" json:"manga"`

	//Config
	TitlePage                  int   `yaml:"title_page" json:"title_page"`
	LimitMb                    int   `yaml:"limit_mb" json:"limit_mb"`
	StripFirstDirectoryFromToc bool  `yaml:"strip_first_directory" json:"strip_first_directory"`
	SortPathMode               int   `yaml:"sort_path_mode" json:"sort_path_mode"`
	Image                      Image `yaml:"image" json:"image"`

	// Other
	Dry          bool   `yaml:"-" json:"dry"`
	DryVerbose   bool   `yaml:"-" json:"dry_verbose"`
	Quiet        bool   `yaml:"-" json:"-"`
	Json         bool   `yaml:"-" json:"-"`
	Strict       bool   `yaml:"-" json:"strict"`
	Workers      int    `yaml:"-" json:"workers"`
	OutputFormat string `yaml:"output_format,omitempty" json:"output_format,omitempty"`
	// InputBytes, when non-nil, is used as the input data instead of reading from Input path.
	// This is used in WASM environments to avoid double-copying through the virtual filesystem.
	InputBytes []byte `yaml:"-" json:"-"`
}

func (o EPUBOptions) WorkersRatio(pct int) (nbWorkers int) {
	nbWorkers = o.Workers * pct / 100
	if nbWorkers < 1 {
		nbWorkers = 1
	}
	return
}

func (o EPUBOptions) ImgStorage() string {
	return o.Output + ".tmp"
}
