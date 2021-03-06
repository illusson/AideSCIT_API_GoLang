package manager

import (
	"SCITEduTool/Application/stdio"
	"SCITEduTool/Application/unit"
	"database/sql"
	"time"
)

type infoManager interface {
	Get(username string) (UserInfo, stdio.MessagedError)
	Update(username string, name string, faculty int, specialty int, class int, grade int) stdio.MessagedError
	SetUserInfoExpired(username string) stdio.MessagedError
}

type infoManagerImpl struct{}

var InfoManager infoManager = infoManagerImpl{}

type ChartItem struct {
	Name string
	ID   int
}

type UserInfo struct {
	Exist     bool
	Expired   bool
	Name      string
	Identify  int
	Level     int
	Grade     int
	Faculty   int
	Specialty int
	Class     int
}

func (infoManagerImpl infoManagerImpl) Get(username string) (UserInfo, stdio.MessagedError) {
	tx, err := unit.Maria.Begin()
	if err != nil {
		stdio.LogWarn(username, "数据库开始事务失败", err)
		return UserInfo{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
	state, err := tx.Prepare("select `u_name`,`u_identify`,`u_level`,`u_faculty`,`u_specialty`,`u_class`,`u_grade`,`u_info_expired` from `user_info` where `u_id`=?")
	if err != nil {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库准备SQL指令失败", err)
		return UserInfo{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
	rows := state.QueryRow(username)
	info := UserInfo{}
	var expired int64
	name := sql.NullString{}
	err = rows.Scan(&name, &info.Identify, &info.Level, &info.Faculty, &info.Specialty, &info.Class, &info.Grade, &expired)
	if err == nil {
		tx.Commit()
		info.Exist = true
		info.Expired = expired < time.Now().Unix()
		if name.Valid {
			info.Name = name.String
		} else {
			info.Name = ""
		}
		return info, stdio.GetEmptyErrorMessage()
	}
	if err == sql.ErrNoRows {
		tx.Commit()
		return UserInfo{}, stdio.GetEmptyErrorMessage()
	} else {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库SQL指令执行失败", err)
		return UserInfo{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
}

func (infoManagerImpl infoManagerImpl) Update(username string, name string, faculty int, specialty int, class int, grade int) stdio.MessagedError {
	exist, errMessage := SessionManager.CheckUserExist(username, "user_info")
	if errMessage.HasInfo {
		return errMessage
	}
	tx, err := unit.Maria.Begin()
	if err != nil {
		stdio.LogWarn(username, "数据库开始事务失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	var state *sql.Stmt
	if !exist {
		state, err = tx.Prepare("insert into `user_info` (`u_id`, `u_name` ,`u_faculty`, `u_specialty`, `u_class`, `u_grade`, `u_info_expired`) values (?, ?, ?, ?, ?, ?, ?)")
	} else {
		state, err = tx.Prepare("update `user_info` set `u_name`=?, `u_faculty`=?, `u_specialty`=?, `u_class`=?, `u_grade`=?, `u_info_expired`=? where `u_id`=?")
	}
	if err != nil {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库准备SQL指令失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	if !exist {
		_, err = state.Exec(username, name, faculty, specialty, class, grade, time.Now().Unix()+1296000)
	} else {
		_, err = state.Exec(name, faculty, specialty, class, grade, time.Now().Unix()+1296000, username)
	}
	if err != nil {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库SQL指令执行失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	tx.Commit()
	if !exist {
		stdio.LogVerbose(username, "向数据库插入新用户信息成功")
	} else {
		stdio.LogVerbose(username, "向数据库更新用户信息成功")
	}
	return stdio.GetEmptyErrorMessage()
}

func (infoManagerImpl infoManagerImpl) SetUserInfoExpired(username string) stdio.MessagedError {
	exist, errMessage := SessionManager.CheckUserExist(username, "user_info")
	if errMessage.HasInfo {
		return errMessage
	}
	if !exist {
		stdio.LogWarn(username, "用户信息不存在", nil)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	tx, err := unit.Maria.Begin()
	if err != nil {
		stdio.LogWarn(username, "数据库开始事务失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	state, err := tx.Prepare("update `user_token` set `u_token_effective`=0 where `u_id`=?")
	if err != nil {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库准备SQL指令失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
	_, err = state.Exec(username)
	if err == nil {
		tx.Commit()
		stdio.LogVerbose(username, "标记用户token失效成功")
		return stdio.GetEmptyErrorMessage()
	} else {
		_ = tx.Rollback()
		stdio.LogWarn(username, "数据库SQL指令执行失败", err)
		return stdio.GetErrorMessage(-500, "请求处理出错")
	}
}
