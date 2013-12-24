package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
)

//-------------mysql --------------------------------------

type IssueLog struct {
	Id            int    `json:"i"`
	Issue_id      int    `json:"id"`
	Issue_subject string `json:"sub"`
	Author        string `json:"author"`
	Project_id    int    `json:"_"`
	Update_on     string `json:"update"`
	Issue_Status  int    `json:"_"`
	ProjectName   string `json:"project"`
}

//Bussiness Layer
type BL struct {
	db *sql.DB
}

func NewBL(db *sql.DB) (b *BL) {
	return &BL{db: db}
}

func (b *BL) CreateTable() (err error) {
	sql := `CREATE TABLE qn_issuselog (
  id int(11) NOT NULL AUTO_INCREMENT,
  issue_id int(11) NOT NULL,
  issue_subject varchar(200) NOT NULL,
  author varchar(45) NOT NULL,
  project_id int(11) NOT NULL,
  update_on datetime NOT NULL,
  issue_status int(11) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY id_UNIQUE (id),
  KEY index3 (issue_status),
  KEY index4 (update_on),
  KEY index5 (issue_id)
) ENGINE=InnoDB AUTO_INCREMENT=1362 DEFAULT CHARSET=utf8`
	_, err = b.db.Exec(sql)
	return
}

func (b *BL) GetIssues(status []int, start string) (item []IssueLog, err error) {
	str := ""
	for _, v := range status {
		str += "," + strconv.Itoa(v)
	}
	str = str[1:]
	sql := fmt.Sprintf("select * from qn_issuselog where issue_status in (%s) and update_on > '%s' order by update_on desc", str, start)
	log.Println(sql)
	rows, err := b.db.Query(sql)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(start)
	for rows.Next() {
		is := IssueLog{}
		err := rows.Scan(&is.Id, &is.Issue_id, &is.Issue_subject, &is.Author, &is.Project_id, &is.Update_on, &is.Issue_Status)
		if err != nil {
			log.Println("time parse error:", err)
		} else {
			item = append(item, is)
		}
	}
	log.Println("end")
	return
}

// 仅查找开发完成且没有发布的issues
func (b *BL) GetNoPubIsses(code_finished_status, published_status []int, start string) (item []IssueLog, err error) {
	strFun := func(status []int) string {
		str := ""
		for _, v := range status {
			str += "," + strconv.Itoa(v)
		}
		str = str[1:]
		return str
	}

	sql := fmt.Sprintf("select * from qn_issuselog where update_on>'%s' and issue_status in (%s) and issue_id not in (select issue_id from qn_issuselog where update_on >'%s' and issue_status in(%s) ) order by update_on desc", start, strFun(code_finished_status), start, strFun(published_status))
	log.Println(sql)
	rows, err := b.db.Query(sql)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(start)
	for rows.Next() {
		is := IssueLog{}
		err := rows.Scan(&is.Id, &is.Issue_id, &is.Issue_subject, &is.Author, &is.Project_id, &is.Update_on, &is.Issue_Status)
		if err != nil {
			log.Println("time parse error:", err)
		} else {
			item = append(item, is)
		}
	}
	log.Println("end")
	return

}

func (b *BL) AddIssues(items []IssueLog) (err error) {
	insert, err := b.db.Prepare("INSERT INTO qn_issuselog (issue_id,issue_subject, author, project_id, update_on, issue_status) VALUES (?,?,?,?,?,?)")
	if err != nil {
		return nil
	}
	for _, item := range items {
		res, err := insert.Exec(item.Issue_id, item.Issue_subject, item.Author, item.Project_id, item.Update_on, item.Issue_Status)
		if err != nil {
			return err
		} else {
			if iid, err := res.LastInsertId(); err == nil {
				log.Println("Insert, Last InsertID:", iid)
			}
		}
	}
	return
}

func (b *BL) Update(id int, item IssueLog) (err error) {

	update, err := b.db.Prepare(`UPDATE qn_issuselog SET  author = ?, project_id = ?, update_on =? WHERE id=?;`)
	if err != nil {
		return nil
	}
	res, err := update.Exec(item.Author, item.Project_id, item.Update_on, id)
	if err == nil {
		if affed, err := res.RowsAffected(); err == nil {
			log.Println("Update, Last Rows Affected:", affed)
		}
	}
	return
}

func (b *BL) Upsert(item IssueLog) (err error) {
	i, er := b.GetIssue(item.Issue_id, item.Issue_Status)
	log.Println(i)
	if er != nil || i.Id == 0 {
		//insert

		err = b.AddIssues([]IssueLog{item})
	} else {
		//update
		if i.Author != item.Author || i.Project_id != item.Project_id || i.Update_on != item.Update_on {
			err = b.Update(i.Id, item)
		} else {
			log.Println("nothing to update")
		}
	}
	return
}

func (b *BL) GetLatest() (item IssueLog, err error) {
	sql := "select * from qn_issuselog where update_on in (select max(update_on)from qn_issuselog)"
	rows, err := b.db.Query(sql)
	if err != nil {
		return
	}
	if rows.Next() {
		err = rows.Scan(&item.Id, &item.Issue_id, &item.Issue_subject, &item.Author, &item.Project_id, &item.Update_on, &item.Issue_Status)
	}
	return
}

func (b *BL) GetIssue(issue_id, status int) (item IssueLog, err error) {
	sql := fmt.Sprintf("select * from qn_issuselog where issue_id=%d and issue_status=%d", issue_id, status)
	fmt.Println("sql:", sql)
	rows, err := b.db.Query(sql)
	if err != nil {
		return
	}
	if rows.Next() {
		err = rows.Scan(&item.Id, &item.Issue_id, &item.Issue_subject, &item.Author, &item.Project_id, &item.Update_on, &item.Issue_Status)
	}
	return
}
