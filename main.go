package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Note struct {
	Color                                         string `json:"color"`
	IsTrashed, IsPinned, IsArchived               bool
	ListContent                                   []ListItem `json:"listContent"`
	TextContent, Title                            string
	Labels                                        []Label      `json:"labels"`
	Annotations                                   []Annotation `json:"annotations"`
	UserEditedTimestampUsec, CreatedTimestampUsec int64
	SourceFile                                    string `json:"-"`
}
type Label struct {
	Name string `json:"name"`
}
type ListItem struct {
	Text      string `json:"text"`
	IsChecked bool   `json:"isChecked"`
}
type Annotation struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (n *Note) UnmarshalJSON(b []byte) error {
	type alias Note
	var raw struct {
		alias
		Labels json.RawMessage `json:"labels"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*n = Note(raw.alias)
	if len(raw.Labels) > 0 {
		var labels []Label
		if json.Unmarshal(raw.Labels, &labels) == nil {
			n.Labels = labels
		} else {
			var names []string
			if err := json.Unmarshal(raw.Labels, &names); err != nil {
				return fmt.Errorf("labels is not an array of strings or named labels")
			}
			for _, name := range names {
				n.Labels = append(n.Labels, Label{Name: name})
			}
		}
	}
	return nil
}

type AnyPage struct {
	SBType   string `json:"sbType"`
	Snapshot struct {
		Data map[string]any `json:"data"`
	} `json:"snapshot"`
}

func uuid() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func restrictions(edit bool) map[string]any {
	r := map[string]any{}
	if edit {
		r["edit"] = true
	}
	r["remove"] = true
	r["drag"] = true
	r["dropOn"] = true
	return r
}

var urlRE = regexp.MustCompile(`https?://[^\s]+`)

func textBlock(text, style string, checked *bool) map[string]any {
	marks := []any{}
	for _, loc := range urlRE.FindAllStringIndex(text, -1) {
		from, to := loc[0], loc[1]
		marks = append(marks, map[string]any{"range": map[string]any{"from": from, "to": to}, "type": "Link", "param": text[from:to]})
	}
	t := map[string]any{"text": text, "marks": map[string]any{"marks": marks}}
	if style != "" {
		t["style"] = style
	}
	if checked != nil {
		t["checked"] = *checked
	}
	return map[string]any{"id": uuid(), "text": t}
}

func convert(n Note, mode string, tagIDs map[string]string) AnyPage {
	hasTitle := n.Title != ""
	typ := "note"
	if mode == "pages" || (mode == "mixed" && hasTitle) {
		typ = "page"
	}
	title := ""
	if typ == "page" {
		title = n.Title
		if title == "" {
			title = time.Unix(0, n.CreatedTimestampUsec*1000).Format("January 02, 2006")
		}
	}
	blocks := []any{}
	children := []string{}
	headerChildren := []string{"featuredRelations"}
	if typ == "page" {
		headerChildren = append(headerChildren, "title", "description")
	}
	header := map[string]any{"id": "header", "restrictions": restrictions(true), "layout": map[string]any{"style": "Header"}, "childrenIds": headerChildren}
	blocks = append(blocks, map[string]any{"id": "", "restrictions": map[string]any{}, "childrenIds": children, "smartblock": map[string]any{}}, header, map[string]any{"id": "featuredRelations", "restrictions": restrictions(false), "featuredRelations": map[string]any{}})
	if n.TextContent != "" {
		for _, line := range strings.Split(n.TextContent, "\n") {
			b := textBlock(line, "", nil)
			blocks = append(blocks, b)
			children = append(children, b["id"].(string))
		}
	}
	for _, item := range n.ListContent {
		b := textBlock(item.Text, "Checkbox", &item.IsChecked)
		blocks = append(blocks, b)
		children = append(children, b["id"].(string))
	}
	for _, a := range n.Annotations {
		b := textBlock(a.Title, "", nil)
		b["text"].(map[string]any)["marks"] = map[string]any{"marks": []any{map[string]any{"range": map[string]any{"to": len(a.Title)}, "type": "Link", "param": a.URL}}}
		blocks = append(blocks, b)
		children = append(children, b["id"].(string))
	}
	var tags []string
	for _, l := range n.Labels {
		if id := tagIDs[l.Name]; id != "" {
			tags = append(tags, id)
		}
	}
	details := map[string]any{"backlinks": []any{}, "createdDate": n.CreatedTimestampUsec / 1000000, "creator": "", "description": "", "featuredRelations": []string{"type"}, "iconEmoji": "", "id": "", "lastModifiedBy": "", "lastModifiedDate": n.UserEditedTimestampUsec / 1000000, "lastOpenedDate": n.UserEditedTimestampUsec / 1000000, "layout": 9, "links": []any{}, "name": title, "restrictions": []any{}, "snippet": "", "sourceFilePath": "", "tag": tags, "type": "ot-note", "workspaceId": ""}
	if typ == "page" {
		details["layout"] = 0
		details["type"] = "ot-page"
		details["featuredRelations"] = []string{"type", "description"}
		blocks = append(blocks, map[string]any{"id": "title", "restrictions": restrictions(false), "fields": map[string]any{"_detailsKey": []string{"name", "done"}}, "text": map[string]any{"style": "Title", "marks": map[string]any{}}}, map[string]any{"id": "description", "restrictions": restrictions(false), "fields": map[string]any{"_detailsKey": "description"}, "text": map[string]any{"style": "Description", "marks": map[string]any{}}})
	}
	blocks[0].(map[string]any)["childrenIds"] = children
	details["sourceFilePath"] = ""
	var p AnyPage
	p.SBType = "Page"
	p.Snapshot.Data = map[string]any{"blocks": blocks, "details": details, "objectTypes": []string{details["type"].(string)}, "relationLinks": []any{}}
	return p
}

func tag(name string) (AnyPage, string) {
	id := uuid()
	d := map[string]any{"addedDate": 0, "apiObjectKey": "tag", "backlinks": []any{}, "createdDate": 0, "creator": "", "id": id, "internalFlags": []any{}, "lastModifiedBy": "", "layout": 13, "links": []any{}, "mentions": []any{}, "name": name, "relationKey": "tag", "relationOptionColor": "orange", "resolvedLayout": 13, "snippet": "", "spaceId": "", "syncDate": 0, "syncError": 0, "type": "ot-relationOption", "uniqueKey": "opt-" + strings.ReplaceAll(id, "-", "")}
	p := AnyPage{SBType: "STRelationOption"}
	p.Snapshot.Data = map[string]any{"blocks": []any{}, "details": d, "objectTypes": []string{"ot-relationOption"}, "relationLinks": []any{}, "key": strings.ReplaceAll(id, "-", "")[:24]}
	return p, id
}
func writeJSON(dir, name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0644)
}
func main() {
	in := flag.String("p", "", "Keep folder")
	out := flag.String("o", "", "output folder")
	mode := flag.String("m", "mixed", "pages or mixed")
	archive := flag.Bool("a", false, "include archive")
	flag.Parse()
	if *in == "" || *out == "" || *in == *out {
		flag.Usage()
		return
	}
	if *mode != "mixed" && *mode != "pages" {
		panic("invalid mode")
	}
	files, _ := filepath.Glob(filepath.Join(*in, "*.json"))
	fmt.Printf("Found %d JSON files in %s\n", len(files), *in)
	var notes []Note
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			panic(e)
		}
		var n Note
		if e = json.Unmarshal(b, &n); e != nil {
			panic(fmt.Errorf("%s: %w", f, e))
		}
		if n.IsArchived && !*archive {
			continue
		}
		n.SourceFile = strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		notes = append(notes, n)
	}
	if len(notes) == 0 {
		panic(errors.New("no notes found"))
	}
	fmt.Printf("Loaded %d notes\n", len(notes))
	_ = os.MkdirAll(*out, 0755)
	ids := map[string]string{}
	var names []string
	for _, n := range notes {
		for _, l := range n.Labels {
			if ids[l.Name] == "" {
				ids[l.Name], _ = func() (string, string) {
					p, id := tag(l.Name)
					name := "tag-" + strings.ReplaceAll(strings.ReplaceAll(l.Name, "/", "_"), "\\", "_") + ".json"
					if err := writeJSON(*out, name, p); err != nil {
						panic(err)
					}
					fmt.Printf("Created tag %q\n", l.Name)
					return id, l.Name
				}()
				names = append(names, l.Name)
			}
		}
	}
	sort.Strings(names)
	for i, n := range notes {
		base := n.SourceFile
		if base == "" {
			base = "note-" + uuid()
		}
		name := base + ".json"
		if err := writeJSON(*out, name, convert(n, *mode, ids)); err != nil {
			panic(err)
		}
		fmt.Printf("[%d/%d] Converted %s\n", i+1, len(notes), name)
	}
	fmt.Printf("Finished: %d notes and %d tags written to %s\n", len(notes), len(ids), *out)
}
