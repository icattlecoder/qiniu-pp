//author:
//	wangming
//	icattlecoder@gmail.com

package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/googollee/go-socket.io"
	"io/ioutil"
	"log"
	_ "mysql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	REDMINEHOST         string `json:"redmine_host"`
	DOING_STATUS        []int  `json:"doing_status"`
	CODE_FINISHEDSTATUS []int  `json:"codefinished_status"`
	PUBLISHED_STATUS    []int  `json:"published_status"`
	LASTMONTHDATE       int    `json:"lastmonth_date"`
	CURRENTMONTHDATE    int    `json:"currentmonth_date"`
	APPKEY              string `json:"app_key"`
	PORT                string `json:"port"`
	DB_USERNAME         string `json:"db_username"`
	DB_PASSWORD         string `json:"db_password"`
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
					if err == nil && (in(int(i), p.config.CODE_FINISHEDSTATUS) || in(int(i), p.config.PUBLISHED_STATUS)) {
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
	date := now.Day()
	lastyear := year
	if date < p.config.CURRENTMONTHDATE {
		mon -= 1
	}
	lastmon := mon - 1
	if mon == 1 {
		lastyear -= 1
		lastmon = 12
	}
	p.dateStr = fmt.Sprintf("%d-%02d-%02d至%d-%02d-%02d", lastyear, lastmon, p.config.LASTMONTHDATE, year, mon, p.config.CURRENTMONTHDATE)
	return "%3E%3C" + fmt.Sprintf("%d-%02d-%02d|%d-%02d-%02d", lastyear, lastmon, p.config.LASTMONTHDATE, year, mon, p.config.CURRENTMONTHDATE)
}

func TimeConv(t string) string {
	d, _ := time.Parse("2006-01-02T15:04:05Z", t)
	return fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second())
}

func (p *pp) Async() {
	bl, err := p.GetBL()
	if err != nil {
		log.Println("Open DB error:", err)
		return
	}
	defer bl.db.Close()
	for {
		isslog_, err := bl.GetLatest()
		log.Println("isslog_:", isslog_)
		if err == nil && isslog_.Id != 0 {
			limitdate := "%3E%3D" + isslog_.Update_on[0:10]
			asy := func(status []int, msg string) {
				for _, v := range status {
					iss := Issues{}
					url := fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on:desc&updated_on=%s", v, 0, limitdate)
					err = p.get(url, &iss)
					for _, vv := range iss.Issues {
						isslog := IssueLog{}
						isslog.Issue_id = vv.Id
						isslog.Update_on = vv.Updated_on
						isslog.Author = p.getAuthor(vv.Id)
						isslog.Project_id = vv.Project.Id
						isslog.Issue_Status = vv.Status.Id
						isslog.Issue_subject = vv.Subject
						isslog.ProjectName = p.getTopProject(vv.Project.Id)
						log.Println("isslog:", isslog)
						err := bl.Upsert(isslog)
						if err == nil {
							isslog.Update_on = vv.Updated_on[0:10]
							p.Notice("msg", isslog)
						}
					}
				}
			}
			asy(p.config.CODE_FINISHEDSTATUS, "ready")
			asy(p.config.PUBLISHED_STATUS, "finished")

		}
		time.Sleep(1e9 * 30)
	}
}

// start="2013-11-01"
func (p *pp) getIssuses(start, end string) {

	limitdate := "%3E%3C" + fmt.Sprintf("%s|%s", start, end)

	{
		p.finished = Issues{}
		for _, v := range p.config.PUBLISHED_STATUS {
			cnt := 0
			iss := Issues{}
			url := fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, 0, limitdate)
			err := p.get(url, &iss)
			if err != nil {
				log.Println(err)
			} else {
				p.finished.Total_count += iss.Total_count
				p.finished.Issues = append(p.finished.Issues, iss.Issues...)
			}
			cnt = len(iss.Issues)
			for cnt < iss.Total_count {
				url = fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
				err = p.get(url, &iss)
				if err != nil {
					log.Println(err)
				} else {
					p.finished.Issues = append(p.finished.Issues, iss.Issues...)
					cnt += len(iss.Issues)
				}
			}
		}

		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			for _, v := range p.finished.Issues {
				isslog := IssueLog{}
				isslog.Issue_id = v.Id
				isslog.Update_on = TimeConv(v.Updated_on)
				isslog.Author = p.getAuthor(v.Id)
				isslog.Project_id = v.Project.Id
				isslog.Issue_Status = v.Status.Id
				isslog.Issue_subject = v.Subject
				bl.Upsert(isslog)
			}
		})()
	}

	{
		p.readyToPub = Issues{}
		for _, v := range p.config.CODE_FINISHEDSTATUS {
			cnt := 0
			iss := Issues{}
			url := fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
			err := p.get(url, &iss)
			if err != nil {
				log.Println(err)
			} else {
				p.readyToPub.Total_count += iss.Total_count
				p.readyToPub.Issues = append(p.readyToPub.Issues, iss.Issues...)
			}
			cnt = len(iss.Issues)
			for cnt < iss.Total_count {
				url = fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
				err = p.get(url, &iss)
				if err != nil {
					log.Println(err)
				} else {
					p.readyToPub.Issues = append(p.readyToPub.Issues, iss.Issues...)
					cnt += len(iss.Issues)
				}
			}
		}
		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			for _, v := range p.readyToPub.Issues {
				isslog := IssueLog{}
				isslog.Update_on = TimeConv(v.Updated_on)
				isslog.Issue_id = v.Id
				isslog.Author = p.getAuthor(v.Id)
				isslog.Project_id = v.Project.Id
				isslog.Issue_Status = v.Status.Id
				isslog.Issue_subject = v.Subject
				bl.Upsert(isslog)
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
	dateStr        string
	lastUpdate     time.Time
}

func (p *pp) getStartQueryTime() string {

	now := time.Now()
	year := now.Year()
	mon := now.Month()
	date := now.Day()
	lastyear := year
	if date < p.config.CURRENTMONTHDATE {
		mon -= 1
	}
	lastmon := mon - 1
	if mon == 1 {
		lastyear -= 1
		lastmon = 12
	}
	return fmt.Sprintf("%d-%02d-%02d", lastyear, lastmon, p.config.LASTMONTHDATE)

}

func (p *pp) _codeFinished(w http.ResponseWriter, r *http.Request) {
	start := p.getStartQueryTime()
	bl, err := p.GetBL()
	if err != nil {
		log.Println("Open DB err")
		return
	}
	defer bl.db.Close()
	iss, err := bl.GetNoPubIsses(p.config.CODE_FINISHEDSTATUS, p.config.PUBLISHED_STATUS, start)
	for k, v := range iss {
		iss[k].ProjectName = p.getTopProject(v.Project_id)
		iss[k].Update_on = iss[k].Update_on[0:10]
	}
	if err != nil {
		log.Println("get Issues from DB error:", err)
	}
	bytes, _ := json.Marshal(iss)
	w.Write(bytes)
}

func (p *pp) _published(w http.ResponseWriter, r *http.Request) {
	start := p.getStartQueryTime()
	bl, err := p.GetBL()
	if err != nil {
		log.Println("Open DB err")
		return
	}
	defer bl.db.Close()
	iss, err := bl.GetIssues(p.config.PUBLISHED_STATUS, start)
	for k, v := range iss {
		iss[k].ProjectName = p.getTopProject(v.Project_id)
		iss[k].Update_on = iss[k].Update_on[0:10]
	}
	if err != nil {
		log.Println("get Issues from DB error:", err)
	}
	bytes, _ := json.Marshal(iss)
	w.Write(bytes)
}

func (p *pp) Notice(msg string, iss interface{}) {
	body, err := json.Marshal(iss)
	if err == nil {
		p.sio.Broadcast(msg, string(body))
		log.Println("Broadcast:", msg, ",body:", string(body))
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
	if in(int(istatus_id), p.config.CODE_FINISHEDSTATUS) || in(int(istatus_id), p.config.PUBLISHED_STATUS) {
		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			is, err := p.getIssue(int(iid))
			if err != nil {
				log.Println(err)
				return
			}
			isslog := IssueLog{}
			isslog.Author = p.getAuthor(int(iid))
			isslog.Issue_id = is.Id
			isslog.Issue_Status = is.Status.Id
			isslog.Issue_subject = is.Subject
			isslog.Project_id = is.Project.Id
			isslog.Update_on = TimeConv(is.Updated_on)
			isslog.ProjectName = p.getTopProject(is.Project.Id)
			log.Println(isslog)
			err = bl.Upsert(isslog)
			if err == nil {
				if in(int(istatus_id), p.config.CODE_FINISHEDSTATUS) {
					p.Notice("ready", isslog)
				} else {
					p.Notice("finished", is)
				}
			}
		})()
	}
}

func (p *pp) init() (err error) {
	p.Projects = make(map[int]string)
	p.getProjects()
	// p.getIssuses()

	return
}

func (p *pp) GetBL() (bl *BL, err error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/qiniupp", p.config.DB_USERNAME, p.config.DB_PASSWORD))
	if err != nil {
		return
	}
	bl = NewBL(db)
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

func (p *pp) fresh(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
		return
	}
	name := r.FormValue("name")
	start := r.FormValue("start")
	end := r.FormValue("end")
	if name == "wangming" && start != "" && end != "" {
		p.getIssuses(start, end)
		w.Write([]byte("async...\n"))
	} else {
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
	}
}

func (p *pp) createTable(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
		return
	}
	name := r.FormValue("name")
	if name == "wangming" {
		bl, err := p.GetBL()
		if err != nil {
			log.Println("Open DB err")
			return
		}
		defer bl.db.Close()
		err = bl.CreateTable()
		if err != nil {
			log.Println("createTable Error:", err)
			w.Write([]byte("createTable Error!"))
		}
	} else {
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
	}
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

	p.sio.HandleFunc("/issues/codefinished", func(w http.ResponseWriter, r *http.Request) { p._codeFinished(w, r) })
	p.sio.HandleFunc("/issues/published", func(w http.ResponseWriter, r *http.Request) { p._published(w, r) })
	p.sio.HandleFunc("/listener", func(w http.ResponseWriter, r *http.Request) { p.Listener(w, r) })
	p.sio.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) { p.proxy(w, r) })
	p.sio.HandleFunc("/async/", func(w http.ResponseWriter, r *http.Request) { p.fresh(w, r) })
	p.sio.HandleFunc("/createTable/", func(w http.ResponseWriter, r *http.Request) { p.createTable(w, r) })
	p.sio.HandleFunc("/getProjects/", func(w http.ResponseWriter, r *http.Request) { p.getProjects() })

	err := p.init()
	if err != nil {
		log.Fatal("server start failed")
	}
	// go p.Async()
	log.Fatal(http.ListenAndServe(":"+p.config.PORT, p.sio))
}

func news(ns *socketio.NameSpace, title, body string, article_num int) {
}
func main() {
	p := New()
	p.Run()
}
