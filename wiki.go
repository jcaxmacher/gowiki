package main

import (
    "database/sql"
	"github.com/russross/blackfriday"
    _ "github.com/mattn/go-sqlite3"
	"html/template"
	"net/http"
	"regexp"
    "log"
    "fmt"
)

var db *sql.DB
var e error

func init() {
    db, e = sql.Open("sqlite3", "./wiki.db")
}


type Page struct {
	Title        string
	Body         []byte
	RenderedBody template.HTML
}

func (p *Page) save() error {
    tx, err := db.Begin()
    if err != nil {
        log.Fatal(err)
    }

    stmt, err := tx.Prepare("insert into pages(name, text) values(?, ?)")
    if err != nil {
        log.Fatal(err)
    }

    a, err := stmt.Exec(p.Title, p.Body)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(a)
    return tx.Commit()
}

func loadPage(title string) (*Page, error) {
    rows, err := db.Query(`
        select id, name, text from pages
        where id in (select max(id) from pages
                     where name = ?)
    `, title)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()
    var body []byte
    for rows.Next() {
        var id int
        var name string
        rows.Scan(&id, &name, &body)
    }
	flags := blackfriday.HTML_SKIP_HTML | blackfriday.HTML_SKIP_STYLE | blackfriday.HTML_TOC | blackfriday.HTML_GITHUB_BLOCKCODE
	exts := blackfriday.EXTENSION_NO_INTRA_EMPHASIS | blackfriday.EXTENSION_TABLES | blackfriday.EXTENSION_FENCED_CODE | blackfriday.EXTENSION_FOOTNOTES | blackfriday.EXTENSION_HEADER_IDS
	renderer := blackfriday.HtmlRenderer(flags, "", "")
	renderedBody := blackfriday.Markdown(body, renderer, exts)

	return &Page{Title: title, Body: body, RenderedBody: template.HTML(renderedBody)}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func main() {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.ListenAndServe(":8080", nil)
}
