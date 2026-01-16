package gui

import (
	"database/sql"
	"sort"
	"strconv"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"ssh-go/internal/db"
	"ssh-go/internal/logging"
	"ssh-go/internal/sshclient"
)

type state struct {
	pageList bool

	nameEd  widget.Editor
	hostEd  widget.Editor
	portEd  widget.Editor
	userEd  widget.Editor
	passEd  widget.Editor
	groupEd widget.Editor

	enableSw widget.Bool
	saveBtn  widget.Clickable

	tabAdd     widget.Clickable
	tabList    widget.Clickable
	refreshBtn widget.Clickable

	showDelSw widget.Bool

	groupList widget.List
	groupBtns []widget.Clickable
	groups    []string

	connList   widget.List
	currentGrp string
	items      []db.Connection

	connectBtns []widget.Clickable
	removeBtns  []widget.Clickable

	sessions       []session
	sessionTabs    []widget.Clickable
	sessionExecBtn []widget.Clickable
	activeSession  int

	lastMessage string
}

type session struct {
	conn  db.Connection
	title string
	log   string
	cmdEd widget.Editor
	term  *sshclient.TerminalSession
}

var invalidateWindow func()

func Start(database *sql.DB, logger *logging.Logger) error {
	go func() {
		w := new(app.Window)
		invalidateWindow = func() { w.Invalidate() }
		w.Option(
			app.Size(unit.Dp(900), unit.Dp(600)),
			app.Title("连接管理"),
		)
		th := material.NewTheme()
		var st state
		st.portEd.SetText("22")
		st.enableSw.Value = true
		st.groupList.Axis = layout.Vertical
		st.connList.Axis = layout.Vertical
		st.pageList = true
		var ops op.Ops
		for {
			e := w.Event()
			switch e := e.(type) {
			case app.DestroyEvent:
				return
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return topBar(gtx, th, database, &st)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								width := gtx.Dp(unit.Dp(220))
								gtx.Constraints.Min.X = width
								gtx.Constraints.Max.X = width
								return sidebar(gtx, th, database, &st)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								if st.pageList {
									return listPage(gtx, th, database, &st)
								}
								return addPage(gtx, th, database, &st)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if st.lastMessage == "" {
							return layout.Dimensions{}
						}
						lbl := material.Body1(th, st.lastMessage)
						return lbl.Layout(gtx)
					}),
				)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
	return nil
}

func addPage(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(16))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Editor(th, &st.nameEd, "名称").Layout),
			layout.Rigid(material.Editor(th, &st.hostEd, "主机").Layout),
			layout.Rigid(material.Editor(th, &st.portEd, "端口").Layout),
			layout.Rigid(material.Editor(th, &st.userEd, "用户名").Layout),
			layout.Rigid(material.Editor(th, &st.passEd, "密码").Layout),
			layout.Rigid(material.Editor(th, &st.groupEd, "分组").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Switch(th, &st.enableSw, "启用").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &st.saveBtn, "保存")
				if st.saveBtn.Clicked(gtx) {
					name := st.nameEd.Text()
					host := st.hostEd.Text()
					user := st.userEd.Text()
					pass := st.passEd.Text()
					group := st.groupEd.Text()
					p, _ := strconv.Atoi(st.portEd.Text())
					en := 0
					if st.enableSw.Value {
						en = 1
					}
					if host == "" || user == "" || pass == "" {
						st.lastMessage = "缺少必填项"
					} else {
						err := db.InsertConnection(database, db.Connection{
							ID:        db.NewID(),
							Name:      name,
							Host:      host,
							Port:      p,
							User:      user,
							Password:  pass,
							GroupName: group,
							Enabled:   en,
							Deleted:   0,
							CreatedAt: time.Now().Unix(),
						})
						if err != nil {
							st.lastMessage = err.Error()
						} else {
							st.lastMessage = "保存成功"
							loadList(database, st)
							st.pageList = true
						}
					}
				}
				return btn.Layout(gtx)
			}),
		)
	})
}

func listPage(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(8))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.List(th, &st.connList)
				return l.Layout(gtx, len(st.items), func(gtx layout.Context, i int) layout.Dimensions {
					it := st.items[i]
					connect := &st.connectBtns[i]
					remove := &st.removeBtns[i]
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(material.Body1(th, it.Name).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.Host).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.Itoa(it.Port)).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.User).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.GroupName).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.Itoa(it.Enabled)).Layout),
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.FormatInt(it.CreatedAt, 10)).Layout),
						layout.Rigid(spacer),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th, connect, "连接")
							if connect.Clicked(gtx) {
								openSession(th, st, it)
							}
							return btn.Layout(gtx)
						}),
						layout.Rigid(spacer),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							text := "删除"
							if it.Deleted == 1 {
								text = "恢复"
							}
							btn := material.Button(th, remove, text)
							if remove.Clicked(gtx) {
								d := 1
								if it.Deleted == 1 {
									d = 0
								}
								if err := db.SetDeleted(database, it.ID, d); err != nil {
									st.lastMessage = err.Error()
								} else {
									loadList(database, st)
								}
							}
							return btn.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(st.sessions) == 0 {
					return layout.Dimensions{}
				}
				return sessionsArea(gtx, th, st)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &st.refreshBtn, "刷新")
				if st.refreshBtn.Clicked(gtx) {
					loadList(database, st)
				}
				return btn.Layout(gtx)
			}),
		)
	})
}

func spacer(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(unit.Dp(8))
	return layout.Dimensions{Size: gtx.Constraints.Min}
}

func loadList(database *sql.DB, st *state) {
	all, err := db.ListConnections(database, st.showDelSw.Value, "")
	if err != nil {
		st.lastMessage = err.Error()
		return
	}

	groupsSet := map[string]struct{}{}
	for _, it := range all {
		groupsSet[it.GroupName] = struct{}{}
	}

	var groups []string
	groups = append(groups, "")
	for g := range groupsSet {
		if g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) > 1 {
		sort.Strings(groups[1:])
	}
	st.groups = groups
	st.groupBtns = make([]widget.Clickable, len(groups))

	valid := false
	for _, g := range groups {
		if g == st.currentGrp {
			valid = true
			break
		}
	}
	if !valid {
		st.currentGrp = ""
	}

	var filtered []db.Connection
	for _, it := range all {
		if st.currentGrp != "" && it.GroupName != st.currentGrp {
			continue
		}
		filtered = append(filtered, it)
	}
	st.items = filtered
	st.connectBtns = make([]widget.Clickable, len(filtered))
	st.removeBtns = make([]widget.Clickable, len(filtered))
}

func topBar(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(8))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &st.tabAdd, "添加连接")
				if !st.pageList {
					btn.Background = th.Palette.ContrastBg
				}
				if st.tabAdd.Clicked(gtx) {
					st.pageList = false
				}
				return btn.Layout(gtx)
			}),
			layout.Rigid(spacer),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &st.tabList, "连接列表")
				if st.pageList {
					btn.Background = th.Palette.ContrastBg
				}
				if st.tabList.Clicked(gtx) {
					st.pageList = true
					loadList(database, st)
				}
				return btn.Layout(gtx)
			}),
			layout.Rigid(spacer),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				before := st.showDelSw.Value
				dims := material.Switch(th, &st.showDelSw, "显示已删除").Layout(gtx)
				if st.showDelSw.Value != before {
					loadList(database, st)
				}
				return dims
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			}),
		)
	})
}

func sidebar(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(8))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.Body1(th, "连接")
				return title.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.List(th, &st.groupList)
				return l.Layout(gtx, len(st.groups), func(gtx layout.Context, i int) layout.Dimensions {
					if i < 0 || i >= len(st.groups) {
						return layout.Dimensions{}
					}
					key := st.groups[i]
					text := key
					if key == "" {
						text = "全部"
					}
					btn := &st.groupBtns[i]
					if btn.Clicked(gtx) {
						st.currentGrp = key
						loadList(database, st)
					}
					return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
						in := layout.UniformInset(unit.Dp(4))
						return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							display := text
							if key == st.currentGrp {
								display = "● " + display
							}
							lbl := material.Body1(th, display)
							return lbl.Layout(gtx)
						})
					})
				})
			}),
		)
	})
}

func openSession(th *material.Theme, st *state, c db.Connection) {
	idx := -1
	for i := range st.sessions {
		if st.sessions[i].conn.ID == c.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		st.sessions = append(st.sessions, session{
			conn:  c,
			title: strconv.Itoa(len(st.sessions)+1) + " " + c.Host,
		})
		st.sessionTabs = append(st.sessionTabs, widget.Clickable{})
		st.sessionExecBtn = append(st.sessionExecBtn, widget.Clickable{})
		idx = len(st.sessions) - 1
	}
	s := &st.sessions[idx]
	s.cmdEd.SingleLine = true
	if s.term == nil {
		if s.log != "" {
			s.log += "\n"
		}
		s.log += "连接主机..."
		term, err := sshclient.NewTerminalSession(c.Host, c.Port, c.User, c.Password, 120, 32)
		if err != nil {
			if s.log != "" {
				s.log += "\n"
			}
			s.log += "连接失败:\n" + err.Error()
			st.lastMessage = "连接失败"
		} else {
			s.term = term
			st.lastMessage = "连接成功"
			startTerminalReader(s)
		}
	}
	st.activeSession = idx
}

func startTerminalReader(s *session) {
	if s == nil || s.term == nil {
		return
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := s.term.Stdout.Read(buf)
			if n > 0 {
				s.log += string(buf[:n])
				if invalidateWindow != nil {
					invalidateWindow()
				}
			}
			if err != nil {
				break
			}
		}
	}()
}

func sessionsArea(gtx layout.Context, th *material.Theme, st *state) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(8))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					func() []layout.FlexChild {
						var children []layout.FlexChild
						for i := range st.sessions {
							i := i
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := &st.sessionTabs[i]
								label := st.sessions[i].title
								return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
									if btn.Clicked(gtx) {
										st.activeSession = i
									}
									pad := layout.UniformInset(unit.Dp(4))
									return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body1(th, label)
										if st.activeSession == i {
											lbl.Font.Weight = 600
										}
										return lbl.Layout(gtx)
									})
								})
							}))
							children = append(children, layout.Rigid(spacer))
						}
						return children
					}()...,
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if st.activeSession < 0 || st.activeSession >= len(st.sessions) {
					return layout.Dimensions{}
				}
				s := &st.sessions[st.activeSession]
				outInset := layout.UniformInset(unit.Dp(4))
				return outInset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body1(th, s.log)
							return lbl.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, material.Editor(th, &s.cmdEd, "输入命令").Layout),
								layout.Rigid(spacer),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if st.activeSession < 0 || st.activeSession >= len(st.sessions) {
										return layout.Dimensions{}
									}
									btn := &st.sessionExecBtn[st.activeSession]
									b := material.Button(th, btn, "执行")
									if btn.Clicked(gtx) {
										cmdText := s.cmdEd.Text()
										if cmdText != "" {
											if s.log != "" {
												s.log += "\n"
											}
											s.log += "> " + cmdText + "\n"
											s.cmdEd.SetText("")
											if invalidateWindow != nil {
												invalidateWindow()
											}
											if s.term != nil && s.term.Stdin != nil {
												_, err := s.term.Stdin.Write([]byte(cmdText + "\n"))
												if err != nil {
													s.log += "错误:\n" + err.Error() + "\n"
													st.lastMessage = "命令发送失败"
												} else {
													st.lastMessage = "命令已发送"
												}
											} else {
												res, err := sshclient.Run(s.conn.Host, s.conn.Port, s.conn.User, s.conn.Password, cmdText)
												if err != nil {
													s.log += "错误:\n" + err.Error() + "\n"
													if res.Stderr != "" {
														s.log += res.Stderr + "\n"
													}
													st.lastMessage = "命令执行失败"
												} else {
													if res.Stdout != "" {
														s.log += res.Stdout
														if res.Stdout[len(res.Stdout)-1] != '\n' {
															s.log += "\n"
														}
													}
													if res.Stderr != "" {
														s.log += res.Stderr
														if res.Stderr[len(res.Stderr)-1] != '\n' {
															s.log += "\n"
														}
													}
													st.lastMessage = "命令执行成功"
												}
											}
										}
									}
									return b.Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
		)
	})
}
