package plugin

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	codespliting "github.com/evanw/esbuild/internal/code_spliting"
	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type TemplateEntry struct {
	FileName string
	Template string
	Chunks   []string
}

type ResolvedTemplateEntry struct {
	TemplatePath   string
	OutputFileName string
	IncludeChunks  []string
}

type HTMLTemplatePluginOptions struct {
	Entrires   []TemplateEntry
	Data       map[string]interface{}
	PublicPath string
}

func HTMLTemplatePlugin(opts HTMLTemplatePluginOptions) api.Plugin {
	return api.Plugin{
		Name: "HTMLTemplatePlugin",
		Setup: func(pb api.PluginBuild) {
			cwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			var resolvedTemplateEntries = make([]ResolvedTemplateEntry, 0, len(opts.Entrires))
			for _, entry := range opts.Entrires {
				reolvedTemplatePath := entry.Template
				outputFileName := entry.FileName
				if outputFileName == "" {
					outputFileName = filepath.Base(entry.Template)
				}
				if !filepath.IsAbs(entry.Template) {
					reolvedTemplatePath = filepath.Join(cwd, entry.Template)
				}
				if !filepath.IsAbs(outputFileName) {
					outputFileName = filepath.Join(cwd, pb.InitialOptions.Outdir, outputFileName)
				}
				resolvedTemplateEntries = append(resolvedTemplateEntries, ResolvedTemplateEntry{
					TemplatePath:   reolvedTemplatePath,
					OutputFileName: outputFileName,
					IncludeChunks:  entry.Chunks,
				})
			}
			var envMap = make(map[string]string)
			for _, env := range os.Environ() {
				values := strings.Split(env, "=")
				envMap[values[0]] = values[1]
			}
			if opts.Data == nil {
				opts.Data = make(map[string]interface{})
			}

			opts.Data["PROCESS_ENV"] = envMap
			opts.Data["HMR"] = pb.InitialOptions.HMR

			pb.OnEnd(func(result *api.BuildResult) (api.OnEndResult, error) {
				var renderTemplate = func(path string) (string, error) {
					data, err := ioutil.ReadFile(path)
					if err != nil {
						return "", err
					}
					content := string(data)
					tpl, err := template.New("tpl").Parse(content)
					if err != nil {
						return "", err
					}
					buf := bytes.NewBuffer([]byte{})
					err = tpl.Execute(buf, opts.Data)
					if err != nil {
						return "", err
					}
					content = buf.String()
					return content, nil
				}
				var iterNode func(*html.Node, func(*html.Node) bool) bool
				iterNode = func(node *html.Node, callback func(node *html.Node) bool) bool {
					continuation := callback(node)
					if !continuation {
						return false
					}
					if node.FirstChild != nil {
						continuation = iterNode(node.FirstChild, callback)
						if !continuation {
							return false
						}
					}
					if node.NextSibling != nil {
						iterNode(node.NextSibling, callback)
						if !continuation {
							return false
						}
					}
					return true
				}
				var replaceNodes = func(node *html.Node, nodes []html.Node) {
					for i := range nodes {
						node.Parent.InsertBefore(&nodes[i], node)
					}
					node.Parent.RemoveChild(node)
				}
				var insertAfter = func(node *html.Node, nodes []html.Node) {
					for i := range nodes {
						node.AppendChild(&nodes[i])
					}
				}
				var appendDependenciesoHTML = func(node *html.Node, htmlPath string, dependencies []api.OutputFile) {
					var scriptOutputCommentNode *html.Node
					var styleOutpuCommenttNode *html.Node
					var headNode *html.Node
					var bodyNode *html.Node
					iterNode(node, func(node *html.Node) bool {
						if node.Type == html.CommentNode {
							if node.Data == "script_output" {
								scriptOutputCommentNode = node
							} else if node.Data == "style_output" {
								styleOutpuCommenttNode = node
							}
						} else if node.Type == html.ElementNode {
							if node.DataAtom == atom.Head {
								headNode = node
							} else if node.DataAtom == atom.Body {
								bodyNode = node
							}
						}
						return true
					})
					var scriptNodes = []html.Node{}
					var styleNodes = []html.Node{}
					for _, file := range dependencies {
						ext := filepath.Ext(file.Path)
						var p string
						var err error
						if opts.PublicPath != "" {
							p = path.Join(opts.PublicPath, filepath.Base(file.Path))
						} else {
							p, err = filepath.Rel(filepath.Dir(htmlPath), file.Path)
							if err != nil {
								panic(err)
							}
						}
						if ext == ".js" {
							scriptNodes = append(scriptNodes, html.Node{
								Type:     html.ElementNode,
								Data:     "script",
								DataAtom: atom.Script,
								Attr: []html.Attribute{
									{
										Key: "src",
										Val: p,
									},
								},
							})
						} else if ext == ".css" {
							styleNodes = append(styleNodes, html.Node{
								Type:     html.ElementNode,
								Data:     "style",
								DataAtom: atom.Style,
								Attr: []html.Attribute{
									{
										Key: "rel",
										Val: "stylesheet",
									},
									{
										Key: "type",
										Val: "text/css",
									},
									{
										Key: "href",
										Val: p,
									},
								},
							})
						}
					}
					if scriptOutputCommentNode != nil {
						replaceNodes(scriptOutputCommentNode, scriptNodes)
					} else if bodyNode != nil {
						insertAfter(scriptOutputCommentNode, scriptNodes)
					}
					if styleOutpuCommenttNode != nil {
						replaceNodes(styleOutpuCommenttNode, styleNodes)
					} else if headNode != nil {
						insertAfter(styleOutpuCommenttNode, styleNodes)
					}
				}
				for _, e := range resolvedTemplateEntries {
					var files = make([]api.OutputFile, 0)
					for _, c := range e.IncludeChunks {
						nodes := codespliting.FindAllChunkDependecies(result.Chunks, c)
						for _, n := range nodes {
							for _, f := range result.OutputFiles {
								if f.ContainChunk(n.Name) {
									files = append(files, f)
								}
							}
						}
					}
					content, err := renderTemplate(e.TemplatePath)
					if err != nil {
						panic(err)
					}
					node, err := html.Parse(bytes.NewBufferString(content))
					if err != nil {
						panic(err)
					}
					appendDependenciesoHTML(node, e.OutputFileName, files)
					var buf bytes.Buffer
					err = html.Render(&buf, node)
					if err != nil {
						panic(err)
					}
					result.OutputFiles = append(result.OutputFiles, api.OutputFile{
						Path:     e.OutputFileName,
						Contents: buf.Bytes(),
					})
				}
				return api.OnEndResult{}, nil
			})
		},
	}
}
