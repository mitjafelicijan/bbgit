package main

import (
	"io"
	"log"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var languageColors = map[string]string{
	"1C Enterprise":     "#814CCC",
	"ActionScript":      "#882B0F",
	"Ada":               "#02f88c",
	"Agda":              "#315665",
	"AGS Script":        "#B9D9FF",
	"Alloy":             "#64C800",
	"AMPL":              "#E6EFBB",
	"Ant Build System":  "#A9157E",
	"ANTLR":             "#9DC3FF",
	"ApacheConf":        "#d12127",
	"Apex":              "#1797c0",
	"API Blueprint":     "#2ACCA8",
	"APL":               "#5A8164",
	"AppleScript":       "#101F1F",
	"Arc":               "#aa2afe",
	"Arduino":           "#bd7911",
	"ASP":               "#6a40fd",
	"AspectJ":           "#a957b0",
	"Assembly":          "#6E4C13",
	"ATS":               "#1ac620",
	"Augeas":            "#9CC134",
	"AutoHotkey":        "#6594b9",
	"AutoIt":            "#1C3552",
	"Awk":               "#c30e9b",
	"Ballerina":         "#FF5000",
	"Batchfile":         "#C1F12E",
	"Befunge":            "#2F2530",
	"Bicep":             "#519aba",
	"Bison":             "#6A463F",
	"BitBake":           "#00bce4",
	"Blade":             "#f7523f",
	"BlitzBasic":        "#00FFAE",
	"BlitzMax":          "#cd6400",
	"Bluespec":          "#12223c",
	"Boo":               "#d4bec1",
	"Brainfuck":         "#2F2530",
	"Brightscript":      "#662D91",
	"C":                 "#555555",
	"C#":                "#178600",
	"C++":               "#f34b7d",
	"C3":                "#2563eb",
	"Caddyfile":         "#22b638",
	"Cairo":             "#ff4a48",
	"Ceylon":            "#dfa535",
	"Chapel":            "#8dc63f",
	"ChucK":             "#3f8000",
	"Cirru":             "#ccccff",
	"Clarion":           "#db901e",
	"Clean":             "#3F85AF",
	"Click":             "#E4E6F3",
	"CLIPS":             "#00A300",
	"Clojure":           "#db5855",
	"CMake":             "#DA3434",
	"COBOL":             "#1d2021",
	"CodeQL":            "#140f46",
	"CoffeeScript":      "#244776",
	"ColdFusion":        "#ed2cd6",
	"Common Lisp":       "#3fb68b",
	"Component Pascal":  "#B0CE4E",
	"Crystal":           "#000100",
	"CSON":              "#244776",
	"Csound":            "#1a1a1a",
	"CSS":               "#563d7c",
	"Cuda":              "#3A4E3A",
	"Curry":             "#531242",
	"Cycript":           "#000000",
	"Cython":            "#fedf5b",
	"D":                 "#ba595e",
	"Dart":              "#00B4AB",
	"DataWeave":         "#003a52",
	"Dhall":             "#dfafff",
	"Dockerfile":        "#384d54",
	"Dogescript":        "#cca760",
	"DTrace":            "#000000",
	"Dylan":             "#6c616e",
	"E":                 "#ccce35",
	"eC":                "#913960",
	"ECL":               "#8a1267",
	"Eiffel":            "#4d6977",
	"EJS":               "#a91e50",
	"Elixir":            "#6e4a7e",
	"Elm":               "#60B5CC",
	"Emacs Lisp":        "#c065db",
	"EmberScript":       "#FFF4F3",
	"EQ":                "#a78649",
	"Erlang":            "#B83998",
	"F#":                "#b845fc",
	"F*":                "#572e30",
	"Factor":            "#636746",
	"Fancy":             "#7b9db4",
	"Fantom":            "#14253c",
	"Faust":             "#c37240",
	"Fennel":            "#fff3d7",
	"fish":              "#4aae47",
	"FLUX":              "#88ccff",
	"Forth":             "#341708",
	"Fortran":           "#4d41b1",
	"FreeBASIC":         "#141AC9",
	"Frege":             "#00cafe",
	"Futhark":           "#5f021f",
	"G-code":            "#D08CF2",
	"Game Maker Language": "#71b417",
	"GAML":              "#FFC766",
	"GAMS":              "#f49a22",
	"GAP":               "#0000cc",
	"GDScript":          "#355570",
	"Genie":             "#fb855d",
	"Genshi":            "#951531",
	"Gentoo Ebuild":     "#9400ff",
	"Gherkin":           "#5B2063",
	"Gleam":             "#ffaff3",
	"GLSL":              "#5686a5",
	"Glyph":             "#c1ac7f",
	"Gnuplot":           "#f0a9f0",
	"Go":                "#00ADD8",
	"Golo":              "#88562A",
	"Gosu":              "#82937f",
	"Grace":             "#615f8b",
	"Gradle":            "#02303a",
	"GraphQL":           "#e10098",
	"Groovy":            "#4298b8",
	"Hack":              "#878787",
	"Haml":              "#ece2a9",
	"Handlebars":        "#f7931e",
	"Harbour":           "#0e60e3",
	"Haskell":           "#5e5086",
	"Haxe":              "#df7900",
	"HCL":               "#844FBA",
	"HiveQL":            "#dce200",
	"HolyC":             "#ffefaf",
	"HTML":              "#e34c26",
	"Hy":                "#7790B2",
	"IDL":               "#a3522f",
	"Idris":             "#b30000",
	"Ignore List":       "#000000",
	"IGOR Pro":          "#0000cc",
	"Imba":              "#16cec6",
	"Inform 7":          "#3d9970",
	"INI":               "#d1dbe0",
	"Inno Setup":        "#264b99",
	"Io":                "#a9188d",
	"Ioke":              "#078193",
	"Isabelle":          "#FEFE00",
	"J":                 "#9EEDFF",
	"Janet":             "#0886a5",
	"Java":              "#b07219",
	"JavaScript":        "#f1e05a",
	"Jinja":             "#a52a22",
	"Jison":             "#56b3cb",
	"Jolie":             "#843179",
	"JSON":              "#292929",
	"Jsonnet":           "#0064bd",
	"Julia":             "#a270ba",
	"Jupyter Notebook":  "#DA5B0B",
	"Just":              "#384d54",
	"Kaitai Struct":     "#773b37",
	"KCL":               "#7ABABF",
	"Kotlin":            "#A97BFF",
	"KRL":               "#28430A",
	"LabVIEW":           "#fede06",
	"Lasso":             "#999999",
	"Latte":             "#f2a542",
	"Lean":              "#3d6117",
	"Less":              "#1d365d",
	"Lex":               "#DBCA00",
	"LigoLANG":          "#0e74ff",
	"LilyPond":          "#9ccc7c",
	"Liquid":            "#67b8de",
	"LiveScript":        "#499886",
	"LLVM":              "#185619",
	"Logtalk":           "#295b9a",
	"LOLCODE":           "#cc9900",
	"LookML":            "#652B81",
	"LSL":               "#3d9970",
	"Lua":               "#000080",
	"Luau":              "#00A2FF",
	"M4":                "#000000",
	"Macaulay2":         "#d8ffff",
	"Makefile":          "#427819",
	"Markdown":          "#083fa1",
	"Marko":             "#42bff2",
	"Mask":              "#f97732",
	"MATLAB":            "#e16737",
	"Max":               "#c4a79c",
	"MAXScript":         "#00a6a6",
	"MDX":               "#fcb32c",
	"Mercury":           "#ff2b2b",
	"Meson":             "#007800",
	"Metal":             "#8f14e9",
	"MiniYAML":          "#ff1111",
	"Mint":              "#02b046",
	"Mirah":             "#c7a938",
	"Modelica":          "#de1d31",
	"Modula-2":          "#10253f",
	"Mojo":              "#ff4c1f",
	"MoonScript":        "#ff4585",
	"Move":              "#4a137a",
	"MQL4":              "#62A8D6",
	"MQL5":              "#4A76B8",
	"MTML":              "#b7e1f4",
	"Mustache":          "#724b3b",
	"Nemerle":           "#3d3c6e",
	"nesC":              "#94B0C7",
	"NetLinx":           "#0aa0ff",
	"NetLogo":           "#ff6375",
	"NewLisp":           "#87AED7",
	"Nextflow":          "#3ac486",
	"Nginx":             "#009639",
	"Nim":               "#ffc200",
	"Nit":               "#009917",
	"Nix":               "#7e7eff",
	"Nushell":           "#4E9906",
	"NWScript":          "#111522",
	"Objective-C":       "#438eff",
	"Objective-C++":     "#6866fb",
	"Objective-J":       "#ff0c5a",
	"OCaml":             "#ef7a08",
	"Odin":              "#60AFFE",
	"Omgrofl":           "#cabbff",
	"ooc":               "#b0b77e",
	"Opal":              "#f7ede0",
	"Open Policy Agent": "#7d9199",
	"OpenCL":            "#ed2e2d",
	"OpenEdge ABL":      "#5ce600",
	"OpenQASM":          "#AA70FF",
	"OpenSCAD":          "#e5cd45",
	"Org":               "#77aa99",
	"Ox":                "#000000",
	"Oxygene":           "#cdd0e3",
	"Oz":                "#fab738",
	"P4":                "#7055b5",
	"Papyrus":           "#660000",
	"Parrot":            "#f3ca0a",
	"Pascal":            "#E3F171",
	"Pawn":              "#dbb284",
	"Pep8":              "#C76F5B",
	"Perl":              "#0298c3",
	"PHP":               "#4f5d95",
	"PicoLisp":          "#6067af",
	"PigLatin":          "#fce7de",
	"Pike":              "#005390",
	"PLpgSQL":           "#336790",
	"PLSQL":             "#dad8d8",
	"PogoScript":        "#d80073",
	"Polar":             "#316880",
	"Pony":              "#000000",
	"PostScript":        "#da291c",
	"PowerShell":        "#012456",
	"Prisma":            "#0c344b",
	"Processing":        "#0096D8",
	"Prolog":            "#74283c",
	"Promela":           "#de3900",
	"Protocol Buffer":   "#000000",
	"Pug":               "#a86454",
	"Puppet":            "#302B6D",
	"PureBasic":         "#5a6986",
	"PureScript":        "#1D222D",
	"Python":            "#3572A5",
	"QMake":             "#000000",
	"QML":               "#44a51c",
	"Qt Script":         "#00b0ff",
	"Quake":             "#882303",
	"R":                 "#198CE7",
	"Racket":            "#3c5caa",
	"Ragel":             "#9d5200",
	"Raku":              "#0000fb",
	"RAML":              "#77d9fb",
	"Razor":             "#512be4",
	"Rebol":             "#358a5b",
	"Red":               "#ee0000",
	"Redcode":           "#000000",
	"Ren'Py":            "#ff7f7f",
	"RenderScript":      "#000000",
	"Rescript":          "#ed4e4e",
	"REXX":              "#d90e09",
	"Ring":              "#2D54CB",
	"Riot":              "#A71E22",
	"RMarkdown":         "#198ce7",
	"RobotFramework":    "#00c0b5",
	"Roff":              "#ecdebe",
	"Rouge":             "#cc0000",
	"Ruby":              "#701516",
	"RUNOFF":            "#660000",
	"Rust":              "#dea584",
	"Sage":              "#000000",
	"SaltStack":         "#646464",
	"SAS":               "#B34936",
	"Sass":              "#a53b70",
	"Scala":             "#c22d40",
	"Scaml":             "#bd181a",
	"Scheme":            "#1e4aec",
	"Scilab":            "#ca0f21",
	"SCSS":              "#c6538c",
	"sed":               "#64b970",
	"Self":              "#0579aa",
	"ShaderLab":         "#222c37",
	"Shell":             "#89e051",
	"Shen":              "#120F14",
	"Sieve":             "#000000",
	"Slash":             "#007eff",
	"Slice":             "#003fa2",
	"Slim":              "#2b2b2b",
	"Smali":             "#000000",
	"Smalltalk":         "#596706",
	"Smarty":            "#f0c040",
	"Smithy":            "#c44536",
	"SmPL":              "#c92223",
	"Solidity":          "#AA6746",
	"SourcePawn":        "#f69e1d",
	"SPARQL":            "#0C4597",
	"SQF":               "#3F3F3F",
	"SQL":               "#e38c00",
	"SQLPL":             "#e38c00",
	"Squirrel":          "#800000",
	"SRecode Template":  "#348a34",
	"Stan":              "#b2011d",
	"Standard ML":       "#dc566d",
	"Starlark":          "#76d275",
	"Stata":             "#1a5f91",
	"STL":               "#373b3e",
	"Stylus":            "#ff6347",
	"SuperCollider":     "#46390b",
	"Svelte":            "#ff3e00",
	"SVG":               "#ff9900",
	"Swift":             "#F05138",
	"SWIG":              "#000000",
	"SystemVerilog":     "#DAE1C2",
	"Tcl":               "#e4cc98",
	"Tcsh":              "#000000",
	"Terra":             "#000000",
	"TeX":               "#3D6117",
	"Thrift":            "#D88E35",
	"TI Program":        "#A0AAAD",
	"TLA":               "#4b0082",
	"TOML":              "#9c4221",
	"TSQL":              "#e38c00",
	"TSX":               "#3178c6",
	"Turing":            "#cf142b",
	"Turtle":            "#EEFF11",
	"Twig":              "#c1d026",
	"TXL":               "#0178b8",
	"TypeScript":        "#3178c6",
	"Typst":             "#239dad",
	"Unified Parallel C": "#4e3617",
	"Unity3D Asset":     "#222c37",
	"Uno":               "#9933cc",
	"UnrealScript":      "#a54c4d",
	"UrWeb":             "#ccc",
	"V":                 "#4f87c4",
	"Vala":              "#fbe5cd",
	"Valve Data Format": "#f26025",
	"VBA":               "#867db1",
	"VBScript":          "#15dcdc",
	"VCL":               "#148AA8",
	"Verilog":           "#b2b7f8",
	"VHDL":              "#adb2cb",
	"Vim Help File":     "#199f4b",
	"Vim Script":        "#199f4b",
	"Visual Basic .NET": "#9400ff",
	"Volt":              "#1F1F1F",
	"Vue":               "#41b883",
	"Vyper":             "#2980b9",
	"WDL":               "#42f1f4",
	"WebAssembly":       "#04133b",
	"WebIDL":            "#000000",
	"Whiley":            "#d5c397",
	"Wikitext":          "#fc5757",
	"Windows Registry Entries": "#52a5df",
	"Witcher Script":    "#ff0000",
	"Wollok":            "#a23738",
	"World of Warcraft Addon Data": "#f7e43a",
	"Wren":              "#383838",
	"X10":               "#4B6BEF",
	"xBase":             "#403a40",
	"XC":                "#99FF33",
	"XML":               "#0060ac",
	"XML Property List": "#0060ac",
	"Xojo":              "#81bd41",
	"Xonsh":             "#285880",
	"XOTL":              "#000000",
	"XQuery":            "#5232e7",
	"XSLT":              "#EB8E35",
	"Xtend":             "#24255d",
	"Yacc":              "#4B6C4B",
	"YAML":              "#cb171e",
	"YANG":              "#000000",
	"YARA":              "#220000",
	"YASnippet":         "#32AB90",
	"ZAP":               "#0d6616",
	"Zeek":              "#000000",
	"ZenScript":         "#00BCD1",
	"Zephir":            "#118f9e",
	"Zig":               "#ec915c",
	"ZIL":               "#dc75e5",
	"Zimpl":             "#d67711",
	"Tree-sitter Query": "#9440ff",
}

func getLanguageStats(repoName string, commitHash string, tree *object.Tree) ([]LanguageStat, error) {
	key := LangCacheKey{RepoName: repoName, CommitHash: commitHash}

	if val, ok := langCache.Load(key); ok {
		log.Printf("OPS Language stats cache hit: %s [%s]", repoName, commitHash)
		return val.([]LanguageStat), nil
	}

	log.Printf("OPS Calculating language stats: %s [%s]", repoName, commitHash)
	start := time.Now()

	type result struct {
		lang string
		size int64
	}

	numWorkers := runtime.NumCPU()
	filesChan := make(chan *object.File, numWorkers*2)
	resultsChan := make(chan result, numWorkers*2)
	var wg sync.WaitGroup

	// Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range filesChan {
				if enry.IsVendor(f.Name) || enry.IsDotFile(f.Name) || enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
					continue
				}

				// Fast path: detect by extension
				lang, safe := enry.GetLanguageByExtension(f.Name)
				if safe && lang != "" && lang != enry.OtherLanguage {
					resultsChan <- result{lang: lang, size: f.Size}
					continue
				}

				// Slow path: detect by content (read only first 16KB)
				reader, err := f.Reader()
				if err != nil {
					continue
				}

				content := make([]byte, 16384)
				n, err := reader.Read(content)
				reader.Close()
				if err != nil && err != io.EOF {
					continue
				}
				content = content[:n]

				if enry.IsBinary(content) {
					continue
				}

				lang = enry.GetLanguage(f.Name, content)
				if lang != "" && lang != enry.OtherLanguage {
					resultsChan <- result{lang: lang, size: f.Size}
				}
			}
		}()
	}

	// Collector
	languages := make(map[string]int64)
	var totalSize int64
	done := make(chan struct{})
	go func() {
		for res := range resultsChan {
			languages[res.lang] += res.size
			totalSize += res.size
		}
		close(done)
	}()

	// Producer
	err := tree.Files().ForEach(func(f *object.File) error {
		filesChan <- f
		return nil
	})
	close(filesChan)
	wg.Wait()
	close(resultsChan)
	<-done

	if err != nil {
		return nil, err
	}

	var stats []LanguageStat
	for name, size := range languages {
		percentage := (float64(size) / float64(totalSize)) * 100
		if percentage < 0.1 {
			continue
		}
		stats = append(stats, LanguageStat{
			Name:       name,
			Size:       size,
			Percentage: percentage,
			Color:      getLanguageColor(name),
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Size > stats[j].Size
	})

	var currentOffset float64
	for i := range stats {
		stats[i].Offset = currentOffset
		currentOffset += stats[i].Percentage
	}

	// Store in cache
	langCache.Store(key, stats)
	NotifySave()

	log.Printf("OPS Language stats calculated for %s [%s] in %v", repoName, commitHash, time.Since(start))

	return stats, nil
}

func getLanguageColor(lang string) string {
	if color, ok := languageColors[lang]; ok {
		return color
	}
	return "#8b8b8b" // Default gray
}
