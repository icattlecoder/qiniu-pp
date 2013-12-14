package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/googollee/go-socket.io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	REDMINEHOST       string `json:"redmine_host"`
	DOING_STATUS      []int  `json:"doing_status"`
	PUBLISHING_STATUS []int  `json:"read2pub_status"`
	FINISHED_STATUS   []int  `json:"finished_status"`
	LASTMONTHDATE     int    `json:"lastmonth_date"`
	CURRENTMONTHDATE  int    `json:"currentmonth_date"`
	APPKEY            string `json:"app_key"`
}

func (p *pp) getRaw(url string) (body []byte, err error) {

	key := "key=" + p.config.APPKEY
	if strings.Contains(url, "?") {
		key = "&" + key
	} else {
		key = "?" + key
	}
	log.Println(p.config.REDMINEHOST + url + key)
	resp, err := http.Get(p.config.REDMINEHOST + url + key)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	return
}

func (p *pp) get(url string, ret interface{}) (err error) {
	body, err := p.getRaw(url)
	err = json.Unmarshal(body, ret)
	return
}

func in(i int, src []int) bool {
	for _, v := range src {
		if v == i {
			return true
		}
	}
	return false
}

//====================================================
func (p *pp) getIssue(uid int) (is Issue_Comm, err error) {
	isw := IssuesWrap{Issue: is}
	err = p.get(fmt.Sprintf("/issues/%d.json", uid), &isw)
	is = isw.Issue
	return
}

func (p *pp) getIssueChangeSets(uid int) string {
	set := IssueChangeSet{}
	err := p.get(fmt.Sprintf("/issues/%d.json?include=journals", uid), &set)
	if err == nil {
		// log.Println("...", set.Issues.Journals)
		for _, v := range set.Issues.Journals {
			for _, vv := range v.Details {
				if vv.Name == "status_id" {
					i, err := strconv.ParseInt(vv.New_value, 10, 32)
					if err == nil && (in(int(i), p.config.PUBLISHING_STATUS) || in(int(i), p.config.FINISHED_STATUS)) {
						rd := strings.NewReader(v.Notes)
						scanner := bufio.NewScanner(rd)
						if scanner.Scan() {
							l := scanner.Text()
							if strings.Contains(l, "* author:") {
								log.Println("author:", l[9:])
								return l[9:]
							}
						}
					}
				}
			}
		}
	} else {
		log.Println(err)
	}
	return ""
}

//获取开发人员
//* autho:<name>
func (p *pp) getAuthor(uid int) string {
	return p.getIssueChangeSets(uid)
}

func (p *pp) getDate() string {
	now := time.Now()
	year := now.Year()
	mon := now.Month()
	lastmon := mon - 1
	lastyear := year
	if mon == 1 {
		lastyear -= 1
		lastmon = 12
	}
	return "%3E%3C" + fmt.Sprintf("%d-%02d-%02d|%d-%02d-%02d", year, lastmon, p.config.LASTMONTHDATE, year, mon, p.config.CURRENTMONTHDATE)
}

func (p *pp) getIssuses() {

	{
		iss := Issues{}

		err := p.get("/issues.json?status_id=5&offset=0&limit=5&sort=updated_on:desc", &iss)
		if err != nil {
			log.Println(err)
		} else {
			p.latestReady = iss
			go (func() {
				for k, v := range p.latestReady.Issues {
					p.latestReady.Issues[k].Author.Name = p.getAuthor(v.Id)
					p.latestReady.Issues[k].Project.Name = p.getTopProject(v.Project.Id)
					p.latestReady.Issues[k].Updated_on = v.Updated_on[0:10]
				}
			})()
		}
	}
	{
		iss := Issues{}
		err := p.get("/issues.json?status_id=80&offset=0&limit=5&sort=updated_on:desc", &iss)
		if err != nil {
			log.Println(err)
		} else {
			p.latestFinished = iss
			go (func() {
				for k, v := range p.latestFinished.Issues {
					p.latestFinished.Issues[k].Author.Name = p.getAuthor(v.Id)
					p.latestFinished.Issues[k].Project.Name = p.getTopProject(v.Project.Id)
					p.latestFinished.Issues[k].Updated_on = v.Updated_on[0:10]
				}
			})()
		}
	}
	limitdate := p.getDate()

	{
		cnt := 0
		iss := Issues{}
		err := p.get(fmt.Sprintf("/issues.json?status_id=5&offset=%d&limit=100&sort=updated_on:desc&updated_on=%s", 0, limitdate), &iss)
		if err != nil {
			log.Println(err)
		} else {
			p.finished = iss
		}
		cnt = len(iss.Issues)
		for cnt < iss.Total_count {
			err = p.get(fmt.Sprintf("/issues.json?status_id=5&offset=%d&limit=100&sort=updated_on:desc&updated_on=%s", cnt, limitdate), &iss)
			if err != nil {
				log.Println(err)
			} else {
				p.finished.Issues = append(p.finished.Issues, iss.Issues...)
				cnt += len(iss.Issues)
			}
		}
		go (func() {
			for k, v := range p.finished.Issues {
				p.finished.Issues[k].Author.Name = p.getAuthor(v.Id)
				p.finished.Issues[k].Project.Name = p.getTopProject(v.Project.Id)
				p.finished.Issues[k].Updated_on = v.Updated_on[0:10]
			}
		})()
	}

	{

		iss := Issues{}
		cnt := 0
		err := p.get(fmt.Sprintf("/issues.json?status_id=80&offset=%d&limit=100&sort=updated_on:desc&updated_on=%s", cnt, limitdate), &iss)
		if err != nil {
			log.Println(err)
		} else {
			p.readyToPub = iss
			p.readyToPub.Issues = append(p.readyToPub.Issues, iss.Issues...)
		}
		cnt = len(iss.Issues)
		for cnt < iss.Total_count {
			err = p.get(fmt.Sprintf("/issues.json?status_id=80&offset=%d&limit=100&sort=updated_on:desc&updated_on=%s", cnt, limitdate), &iss)
			if err != nil {
				log.Println(err)
			} else {
				cnt += len(iss.Issues)
				p.readyToPub.Issues = append(p.readyToPub.Issues, iss.Issues...)
			}
		}

		go (func() {
			for k, v := range p.readyToPub.Issues {
				p.readyToPub.Issues[k].Author.Name = p.getAuthor(v.Id)
				p.readyToPub.Issues[k].Project.Name = p.getTopProject(v.Project.Id)
				p.readyToPub.Issues[k].Updated_on = v.Updated_on[0:10]
			}
		})()
	}
	return
}

func (p *pp) r(id int) string {
	var n string
	for _, v := range p.rawprojects.Projects {
		if v.Id == id && v.Parent.Id > 0 {
			return p.r(v.Parent.Id)
		} else if v.Id == id {
			n = v.Name
			p.Projects[id] = n
			break
		}
	}
	return n
}

func (p *pp) getTopProject(id int) string {
	if v, ok := p.Projects[id]; ok {
		return v
	}
	return p.r(id)
}

func (p *pp) getProjects() {
	ps := Projects{}
	err := p.get("/projects.json", &ps)
	if err != nil {
		log.Println(err)
	} else {
		p.rawprojects = ps
	}
}

//====================================================
type pp struct {
	config         *Config
	issues         Issues
	latestReady    Issues
	latestFinished Issues
	readyToPub     Issues
	finished       Issues
	rawprojects    Projects
	Projects       map[int]string
	sio            *socketio.SocketIOServer
}

func setCORSHeaders(w http.ResponseWriter, req *http.Request) {
	h := w.Header()
	h.Set("Pragma", "no-cache")
	h.Set("Cache-Control", "no-store, no-cache, must-revalidate")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "OPTIONS, HEAD, POST")
	if requests, ok := req.Header["Access-Control-Request-Headers"]; ok {
		h.Set("Access-Control-Allow-Headers", requests[0])
	} else {
		h.Set("Access-Control-Allow-Headers", "X-File-Name, X-File-Type, X-File-Size")
	}
}

func (p *pp) IssueHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	// var bytes []byte
	// if strings.Contains(r.URL.Path, "ready2pub") {
	// } else if strings.Contains(r.URL.Path, "finished") {
	// 	bytes, _ = json.Marshal(p.finished)
	// } else if strings.Contains(r.URL.Path, "lastedready") {
	// 	bytes, _ = json.Marshal(p.latestReady)
	// } else if strings.Contains(r.URL.Path, "lastedfinished") {
	// 	log.Println("........................................")
	// 	bytes, _ = json.Marshal(p.latestFinished)
	// }
	// res := string(bytes)
	// // w.Write(res)
	// fmt.Fprintf(w, res)
}

func (p *pp) _ready2pub(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(p.readyToPub)
	res := string(bytes)
	// w.Write(res)
	fmt.Fprintf(w, res)
}
func (p *pp) _finished(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(p.finished)
	res := string(bytes)
	// w.Write(res)
	fmt.Fprintf(w, res)
}
func (p *pp) _lastedready(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(p.latestReady)
	res := string(bytes)
	// w.Write(res)
	fmt.Fprintf(w, res)
}
func (p *pp) _lastedfinished(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(p.latestFinished)
	res := string(bytes)
	// w.Write(res)
	fmt.Fprintf(w, res)
}

func (p *pp) Notice(msg string, iss Issue_Comm) {
	body, err := json.Marshal(iss)
	if err == nil {
		p.sio.Broadcast(msg, string(body))
	}
}

func (p *pp) Listener(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	id := r.FormValue("id")
	status_id := r.FormValue("status_id")
	iid, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Println(err)
		return
	}
	istatus_id, err := strconv.ParseInt(status_id, 10, 32)
	if err != nil {
		log.Println(err)
		return
	}
	if in(int(istatus_id), p.config.PUBLISHING_STATUS) {
		go (func() {
			is, err := p.getIssue(int(iid))
			if err != nil {
				log.Println(err)
				return
			}
			is.Author.Name = p.getAuthor(int(iid))
			is.Updated_on = is.Updated_on[0:10]
			p.latestReady.Pop()
			p.latestReady.Push(is)
			p.Notice("ready", is)
		})()

	} else if in(int(istatus_id), p.config.FINISHED_STATUS) {
		go (func() {
			is, err := p.getIssue(int(iid))
			if err != nil {
				log.Println(err)
				return
			}
			is.Author.Name = p.getAuthor(int(iid))
			is.Updated_on = is.Updated_on[0:10]
			p.latestFinished.Pop()
			p.latestFinished.Push(is)
			p.Notice("finished", is)
		})()
	}
}

func (p *pp) debugloop() {
	for {
		time.Sleep(1e9 * 10)
		iss := Issues{}
		err := p.get("/issues.json?status_id=5&offset=0&limit=1&sort=updated_on:desc", &iss)
		if err == nil && len(iss.Issues) > 0 && iss.Issues[0].Id != p.latestReady.Issues[0].Id {
			p.latestReady.Issues[0].Author.Name = p.getAuthor(iss.Issues[0].Id)
			p.latestReady.Issues[0].Project.Name = p.getTopProject(iss.Issues[0].Id)
			p.latestReady.Issues[0].Updated_on = iss.Issues[0].Updated_on[0:10]
			_, err := json.Marshal(iss.Issues[0])
			if err == nil {
				// p.sio.Broadcast("news", string(bs))
			}
		}
	}
}

func (p *pp) init() (err error) {
	p.Projects = make(map[int]string)
	p.getProjects()
	p.getIssuses()

	return
}

func New() (p pp) {
	body, err := ioutil.ReadFile("./pp.json")
	if err != nil {
		fmt.Println(`
pp.json:
{
    "redmine_host": "http://pm.qbox.me/redmine",
    "doing_status": [84],
    "read2pub_status": [80],
    "finished_status": [5],
    "lastmonth_date": 1,
    "currentmonth_date": 1
}`)

	}
	config := Config{}
	err = json.Unmarshal(body, &config)
	if err != nil {
		fmt.Println("config file error")
		os.Exit(-1)
	}
	log.Println(config)
	p = pp{config: &config}
	return
}

func (p *pp) proxy(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	body, err := p.getRaw(r.URL.Path[6:])
	if err != nil {
		log.Println(err)
	}
	fmt.Fprintf(w, string(body))
}

func staticDirHandler(mux *socketio.SocketIOServer, prefix string, staticDir string, flags int) {
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		file := staticDir + r.URL.Path[len(prefix)-1:]
		log.Print(file)
		if (flags) == 0 {
			fi, err := os.Stat(file)
			if err != nil || fi.IsDir() {
				http.NotFound(w, r)
				return
			}
		}
		http.ServeFile(w, r, file)
	})
}

func (p *pp) Run() {

	sock_config := &socketio.Config{}
	sock_config.HeartbeatTimeout = 2
	sock_config.ClosingTimeout = 4

	p.sio = socketio.NewSocketIOServer(sock_config)

	p.sio.Of("/pol").On("news", news)
	p.sio.Of("/pol").On("ping", func(ns *socketio.NameSpace) {
		p.sio.In("/pol").Broadcast("pong", nil)
	})

	staticDirHandler(p.sio, "/static/", "static", 0)

	p.sio.HandleFunc("/issues/", func(w http.ResponseWriter, r *http.Request) { p.IssueHandler(w, r) })
	p.sio.HandleFunc("/issues/ready2pub", func(w http.ResponseWriter, r *http.Request) { p._ready2pub(w, r) })
	p.sio.HandleFunc("/issues/finished", func(w http.ResponseWriter, r *http.Request) { p._finished(w, r) })
	p.sio.HandleFunc("/issues/lastedready", func(w http.ResponseWriter, r *http.Request) { p._lastedready(w, r) })
	p.sio.HandleFunc("/issues/lastedfinished", func(w http.ResponseWriter, r *http.Request) { p._lastedfinished(w, r) })
	p.sio.HandleFunc("/listener/", func(w http.ResponseWriter, r *http.Request) { p.Listener(w, r) })
	p.sio.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) { p.proxy(w, r) })

	err := p.init()
	if err != nil {
		log.Fatal("server start failed")
	}
	log.Fatal(http.ListenAndServe(":8080", p.sio))
}

func news(ns *socketio.NameSpace, title, body string, article_num int) {
}
func main() {
	p := New()
	p.Run()
}
