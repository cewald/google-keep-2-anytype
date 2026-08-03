package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type Memo struct {
	State      string `json:"state"`
	Content    string `json:"content"`
	Visibility string `json:"visibility"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
	Pinned     bool   `json:"pinned"`
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
	r := map[string]any{"remove": true, "drag": true, "dropOn": true}
	if edit {
		r["edit"] = true
	}
	return r
}

var urlRE = regexp.MustCompile(`https?://[^\s]+`)

func anyText(text, style string, checked bool) map[string]any {
	if style == "" {
		style = "Paragraph"
	}
	marks := []any{}
	for _, p := range urlRE.FindAllStringIndex(text, -1) {
		marks = append(marks, map[string]any{"range": map[string]any{"from": p[0], "to": p[1]}, "type": "Link", "param": text[p[0]:p[1]]})
	}
	return map[string]any{"id": uuid(), "fields": nil, "restrictions": restrictions(false), "childrenIds": []string{}, "text": map[string]any{"text": text, "style": style, "marks": map[string]any{"marks": marks}, "checked": checked, "color": "", "iconEmoji": "", "iconImage": ""}}
}
func convertAnytype(n Note, tagIDs map[string]string) AnyPage {
	blocks := []any{}
	children := []string{"header"}
	blocks = append(blocks, map[string]any{"id": "", "restrictions": map[string]any{}, "childrenIds": children}, map[string]any{"id": "header", "restrictions": restrictions(true), "layout": map[string]any{"style": "Header"}, "childrenIds": []string{"featuredRelations", "title", "description"}}, map[string]any{"id": "featuredRelations", "restrictions": restrictions(false), "featuredRelations": map[string]any{}})
	add := func(b map[string]any) { blocks = append(blocks, b); children = append(children, b["id"].(string)) }
	for _, line := range strings.Split(n.TextContent, "\n") {
		if strings.TrimSpace(line) != "" {
			add(anyText(line, "", false))
		}
	}
	for _, item := range n.ListContent {
		add(anyText(item.Text, "Checkbox", item.IsChecked))
	}
	for _, a := range n.Annotations {
		b := anyText(a.Title, "", false)
		b["text"].(map[string]any)["marks"] = map[string]any{"marks": []any{map[string]any{"range": map[string]any{"to": len(a.Title)}, "type": "Link", "param": a.URL}}}
		add(b)
	}
	tags := []string{}
	for _, l := range n.Labels {
		if id := tagIDs[l.Name]; id != "" {
			tags = append(tags, id)
		}
	}
	blocks[0].(map[string]any)["childrenIds"] = children
	blocks = append(blocks, map[string]any{"id": "title", "restrictions": restrictions(false), "fields": map[string]any{"_detailsKey": []string{"name", "done"}}, "text": map[string]any{"text": n.Title, "style": "Title", "marks": map[string]any{}}}, map[string]any{"id": "description", "restrictions": restrictions(false), "fields": map[string]any{"_detailsKey": "description"}, "text": map[string]any{"style": "Description", "marks": map[string]any{}}})
	p := AnyPage{SBType: "Page"}
	p.Snapshot.Data = map[string]any{"blocks": blocks, "details": map[string]any{"name": n.Title, "tag": tags, "type": "ot-page", "layout": 0, "createdDate": n.CreatedTimestampUsec / 1000000, "lastModifiedDate": n.UserEditedTimestampUsec / 1000000}, "objectTypes": []string{"ot-page"}, "relationLinks": []any{}}
	return p
}
func anyTag(name string) (AnyPage, string) {
	id := uuid()
	p := AnyPage{SBType: "STRelationOption"}
	p.Snapshot.Data = map[string]any{"blocks": []any{}, "details": map[string]any{"id": id, "name": name, "type": "ot-relationOption"}, "objectTypes": []string{"ot-relationOption"}, "relationLinks": []any{}}
	return p, id
}

func convert(n Note) Memo {
	var lines []string
	if n.Title != "" {
		lines = append(lines, "# "+n.Title, "")
	}
	if n.TextContent != "" {
		lines = append(lines, n.TextContent)
	}
	for _, item := range n.ListContent {
		checked := " "
		if item.IsChecked {
			checked = "x"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", checked, item.Text))
	}
	for _, a := range n.Annotations {
		lines = append(lines, fmt.Sprintf("[%s](%s)", a.Title, a.URL))
	}
	for _, l := range n.Labels {
		lines = append(lines, "#"+strings.ReplaceAll(l.Name, " ", "-"))
	}
	created := time.Unix(0, n.CreatedTimestampUsec*1000).UTC().Format(time.RFC3339)
	updated := time.Unix(0, n.UserEditedTimestampUsec*1000).UTC().Format(time.RFC3339)
	state := "NORMAL"
	if n.IsArchived {
		state = "ARCHIVED"
	}
	return Memo{State: state, Content: strings.TrimSpace(strings.Join(lines, "\n")), Visibility: "PRIVATE", CreateTime: created, UpdateTime: updated, Pinned: n.IsPinned}
}
func writeJSON(dir, name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0644)
}

func importMemo(client *http.Client, host, token string, memo Memo) error {
	payload, err := json.Marshal(memo)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(host, "/")+"/api/v1/memos", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Memos returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if !memo.Pinned {
		return nil
	}
	var created struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return fmt.Errorf("decode create response: %w", err)
	}
	name := strings.TrimPrefix(created.Name, "memos/")
	if name == "" {
		return errors.New("Memos create response did not include a memo name")
	}
	patchBody, _ := json.Marshal(map[string]bool{"pinned": true})
	patchURL := strings.TrimRight(host, "/") + "/api/v1/memos/" + url.PathEscape(name) + "?updateMask=pinned"
	patch, err := http.NewRequest(http.MethodPatch, patchURL, bytes.NewReader(patchBody))
	if err != nil {
		return err
	}
	patch.Header.Set("Authorization", "Bearer "+token)
	patch.Header.Set("Content-Type", "application/json")
	patchResp, err := client.Do(patch)
	if err != nil {
		return err
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode < 200 || patchResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(patchResp.Body, 4096))
		return fmt.Errorf("Memos pin update returned %s: %s", patchResp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func main() {
	archive := flag.Bool("a", false, "include archive")
	format := flag.String("format", "memos", "output format: memos or anytype")
	host := flag.String("host", os.Getenv("MEMOS_HOST"), "Memos host, for example https://memos.example.com")
	token := flag.String("access-token", os.Getenv("MEMOS_ACCESS_TOKEN"), "Memos access token")
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 || args[0] == args[1] {
		flag.Usage()
		return
	}
	in, out := args[0], args[1]
	if *format != "memos" && *format != "anytype" {
		panic("invalid format")
	}

	files, _ := filepath.Glob(filepath.Join(in, "*.json"))
	fmt.Printf("Found %d JSON files in %s\n", len(files), in)
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
	_ = os.MkdirAll(out, 0755)
	tagIDs := map[string]string{}
	if *format == "anytype" {
		var names []string
		for _, n := range notes {
			for _, l := range n.Labels {
				if tagIDs[l.Name] == "" {
					p, id := anyTag(l.Name)
					tagIDs[l.Name] = id
					name := "tag-" + strings.ReplaceAll(l.Name, "/", "_") + ".json"
					if err := writeJSON(out, name, p); err != nil {
						panic(err)
					}
					names = append(names, l.Name)
				}
			}
		}
		sort.Strings(names)
	}
	if *format == "anytype" && (*host != "" || *token != "") {
		panic("-host and -access-token are only supported with -format memos")
	}
	if (*host == "") != (*token == "") {
		panic("-host and -access-token must be provided together")
	}
	client := &http.Client{}
	if *host != "" {
		fmt.Printf("Importing into %s\n", strings.TrimRight(*host, "/"))
	}
	for i, n := range notes {
		base := n.SourceFile
		if base == "" {
			base = "note-" + fmt.Sprint(i+1)
		}
		name := base + ".json"
		var output any
		var memo Memo
		if *format == "anytype" {
			output = convertAnytype(n, tagIDs)
		} else {
			memo = convert(n)
			output = memo
		}
		if err := writeJSON(out, name, output); err != nil {
			panic(err)
		}
		if *format == "memos" && *host != "" {
			if err := importMemo(client, *host, *token, memo); err != nil {
				panic(fmt.Errorf("%s: %w", name, err))
			}
		}
		fmt.Printf("[%d/%d] Converted %s\n", i+1, len(notes), name)
	}
	fmt.Printf("Finished: %d %s written to %s\n", len(notes), *format, out)
}
