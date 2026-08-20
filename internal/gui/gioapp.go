package gui // 本文件所在的包名，GUI 相关代码都在这个包里

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/text/encoding/simplifiedchinese"
	"boltshell/internal/db"
	"boltshell/internal/logging"
	"boltshell/internal/sshclient"

	"github.com/atotto/clipboard"
)

type state struct { // 整个窗口的全局状态，保存所有控件和数据的当前值
	pageList bool // 当前是否显示“连接列表”页；true=列表页，false=添加连接页

	nameEd  widget.Editor // “名称”输入框的编辑器状态
	hostEd  widget.Editor // “主机”输入框的编辑器状态
	portEd  widget.Editor // “端口”输入框的编辑器状态
	userEd  widget.Editor // “用户名”输入框的编辑器状态
	passEd  widget.Editor // “密码”输入框的编辑器状态
	groupEd widget.Editor // “分组”输入框的编辑器状态

	enableSw widget.Bool      // “启用”开关的状态（true=启用，false=禁用）
	saveBtn  widget.Clickable // “保存”按钮的点击状态

	tabAdd     widget.Clickable // 顶部“添加连接”标签按钮点击状态
	tabList    widget.Clickable // 顶部“连接列表”标签按钮点击状态
	refreshBtn widget.Clickable // 底部“刷新”按钮点击状态

	showDelSw widget.Bool // “显示已删除”开关状态（控制是否显示逻辑删除的连接）

	groupList widget.List        // 左侧分组列表的滚动 List 状态
	groupBtns []widget.Clickable // 每个分组一颗按钮，用于响应点击
	groups    []string           // 所有分组名称（第一个为空字符串代表“全部”）

	connList   widget.List     // 中间连接列表的滚动 List 状态
	currentGrp string          // 当前选中的分组名（空字符串表示“全部”）
	items      []db.Connection // 当前按照分组和删除标记过滤后的连接列表

	connectBtns []widget.Clickable // “连接”按钮列表，与 items 一一对应
	removeBtns  []widget.Clickable // “删除/恢复”按钮列表，与 items 一一对应

	sessions      []session          // 已打开的 SSH 会话列表（一个会话对应一个终端标签页）
	sessionTabs   []widget.Clickable // 顶部每个会话 Tab 对应的 Clickable
	activeSession int                // 当前激活的会话索引（-1 表示没有激活会话）

	lastMessage string // 底部状态栏要显示的提示信息（如“保存成功”“连接失败”等）

	showConnListDialog   bool
	connListDialogOK     widget.Clickable
	connListDialogCancel widget.Clickable

	focusRequested bool // 请求在下一帧聚焦当前会话
}

type session struct { // 单个 SSH 会话（终端标签页）的状态
	conn  db.Connection
	title string
	log   string

	term *sshclient.TerminalSession

	logList layout.List
	cmdEd   widget.Editor

	menuArea        widget.Clickable
	menuCopyBtn     widget.Clickable
	menuPasteBtn    widget.Clickable
	contextMenuOpen bool
	tag             *int
}

var guiLogger *logging.Logger
var invalidateWindow func()

func Start(database *sql.DB, logger *logging.Logger) error { // GUI 程序入口，由 main 调用
	guiLogger = logger
	go func() { // 启动一个 goroutine 运行窗口事件循环
		w := new(app.Window)                         // 创建一个新的 Gio 窗口
		invalidateWindow = func() { w.Invalidate() } // 记录重绘函数，供其他地方调用
		w.Option(                                    // 设置窗口的一些属性
			app.Size(unit.Dp(900), unit.Dp(600)), // 窗口尺寸为 900x600 dp
			app.Title("连接管理"),                    // 窗口标题文字
		)
		w.Option(app.Maximized.Option())
		th := material.NewTheme()           // 创建一个 Material Design 主题
		var st state                        // 界面全局状态（所有控件的状态都存在这里）
		st.portEd.SetText("22")             // “端口”输入框默认值设为 22
		st.enableSw.Value = true            // “启用”开关默认勾选
		st.groupList.Axis = layout.Vertical // 左侧分组列表按垂直方向滚动
		st.connList.Axis = layout.Vertical  // 中间连接列表按垂直方向滚动
		loadList(database, &st)             // 启动时先从数据库加载一次连接列表
		st.pageList = true                  // 默认显示“连接列表”页面
		var ops op.Ops                      // 存放绘制操作的对象，每一帧复用
		for {                               // 事件循环，不断处理窗口事件
			e := w.Event()         // 获取一个事件（可能是重绘、关闭等）
			switch e := e.(type) { // 按事件具体类型分支
			case app.DestroyEvent: // 窗口被关闭
				return // 结束 goroutine
			case app.FrameEvent: // 需要绘制一帧界面
				gtx := app.NewContext(&ops, e) // 构造布局上下文，用于布局和绘制
				layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
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
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if !st.showConnListDialog {
							return layout.Dimensions{}
						}
						return drawConnListDialog(gtx, th, database, &st)
					}),
				)
				e.Frame(gtx.Ops) // 提交本帧的绘制操作到窗口
			}
		}
	}()
	app.Main() // 进入 Gio 主循环，阻塞直到窗口关闭
	return nil // 正常结束时返回 nil
}

func addPage(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions { // “添加连接”页面布局
	inset := layout.UniformInset(unit.Dp(16)) // 整个页面四周留 16dp 内边距
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, // 垂直依次排布各个输入控件
			layout.Rigid(material.Editor(th, &st.nameEd, "名称").Layout),  // 连接名称
			layout.Rigid(material.Editor(th, &st.hostEd, "主机").Layout),  // 主机地址
			layout.Rigid(material.Editor(th, &st.portEd, "端口").Layout),  // 端口
			layout.Rigid(material.Editor(th, &st.userEd, "用户名").Layout), // 用户名
			layout.Rigid(material.Editor(th, &st.passEd, "密码").Layout),  // 密码
			layout.Rigid(material.Editor(th, &st.groupEd, "分组").Layout), // 分组
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Switch(th, &st.enableSw, "启用").Layout(gtx) // 是否启用
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &st.saveBtn, "保存") // 保存按钮
				if st.saveBtn.Clicked(gtx) {                  // 点击后执行保存逻辑
					name := st.nameEd.Text()
					host := st.hostEd.Text()
					user := st.userEd.Text()
					pass := st.passEd.Text()
					group := st.groupEd.Text()
					p, _ := strconv.Atoi(st.portEd.Text()) // 端口文本转为整数，错误时默认为 0
					en := 0
					if st.enableSw.Value { // “启用”开关转为 0/1 存入数据库
						en = 1
					}
					if host == "" || user == "" || pass == "" { // 必填项校验
						st.lastMessage = "缺少必填项"
					} else {
						err := db.InsertConnection(database, db.Connection{ // 插入一条新的连接记录
							ID:        db.NewID(),        // 主键 ID（非自增）
							Name:      name,              // 名称
							Host:      host,              // 主机
							Port:      p,                 // 端口
							User:      user,              // 用户名
							Password:  pass,              // 密码
							GroupName: group,             // 分组名称
							Enabled:   en,                // 是否启用（1=启用，0=禁用）
							Deleted:   0,                 // 默认未删除
							CreatedAt: time.Now().Unix(), // 创建时间（秒级时间戳）
						})
						if err != nil { // 保存失败
							st.lastMessage = err.Error()
						} else { // 保存成功
							st.lastMessage = "保存成功"
							loadList(database, st) // 重新加载列表页数据
							st.pageList = true     // 自动切回“连接列表”页面
						}
					}
				}
				return btn.Layout(gtx)
			}),
		)
	})
}

func listPage(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions { // “连接列表”页面布局
	inset := layout.UniformInset(unit.Dp(8)) // 页面四周 8dp 内边距
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 会话 Tab 区域
				if len(st.sessions) == 0 {
					return layout.Dimensions{}
				}
				return sessionTabsBar(gtx, th, st)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 底部：终端区域
				if len(st.sessions) == 0 { // 没有打开任何会话则不显示
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Body1(th, "请点击上方 [连接列表] 打开会话").Layout(gtx)
					})
				}
				return sessionsArea(gtx, th, st)
			}),
		)
	})
}

func spacer(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(unit.Dp(8))
	return layout.Dimensions{Size: gtx.Constraints.Min}
}

func loadList(database *sql.DB, st *state) { // 从数据库加载连接列表并应用过滤
	// 根据“显示已删除”开关的状态，决定是否加载 deleted=1 的记录
	all, err := db.ListConnections(database, st.showDelSw.Value, "")
	if err != nil { // 如果查询出错
		st.lastMessage = err.Error() // 在状态栏显示错误信息
		return
	}

	// 统计所有出现过的分组名称，用于构建左侧分组列表
	groupsSet := map[string]struct{}{}
	for _, it := range all {
		groupsSet[it.GroupName] = struct{}{}
	}

	// 将 Set 转换为 Slice，并排序
	var groups []string
	groups = append(groups, "") // 第一个元素为空字符串，表示“全部”分组
	for g := range groupsSet {
		if g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) > 1 {
		sort.Strings(groups[1:]) // 对除了第一个以外的分组名进行字母排序
	}
	st.groups = groups                                   // 更新状态中的分组列表
	st.groupBtns = make([]widget.Clickable, len(groups)) // 重置分组按钮状态数组

	// 检查当前选中的分组是否还在新的分组列表中
	valid := false
	for _, g := range groups {
		if g == st.currentGrp {
			valid = true
			break
		}
	}
	if !valid { // 如果当前选中的分组不存在了（例如被改名或删除了），则重置为“全部”
		st.currentGrp = ""
	}

	// 根据当前选中的分组过滤连接列表
	var filtered []db.Connection
	for _, it := range all {
		if st.currentGrp != "" && it.GroupName != st.currentGrp { // 如果没选中“全部”且分组名不匹配
			continue // 跳过该记录
		}
		filtered = append(filtered, it) // 加入过滤后的列表
	}
	st.items = filtered                                      // 更新界面显示的列表数据
	st.connectBtns = make([]widget.Clickable, len(filtered)) // 重置连接按钮状态
	st.removeBtns = make([]widget.Clickable, len(filtered))  // 重置删除按钮状态
}

func topBar(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions { // 顶部工具栏布局
	inset := layout.UniformInset(unit.Dp(8)) // 四周留白 8dp
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, // 水平排列按钮
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // “添加连接”按钮
				btn := material.Button(th, &st.tabAdd, "添加连接2")
				if !st.pageList { // 如果当前已经在添加页，高亮或改变背景
					btn.Background = th.Palette.ContrastBg
				}
				if st.tabAdd.Clicked(gtx) { // 点击时切换到添加页
					st.pageList = false
				}
				return btn.Layout(gtx)
			}),
			layout.Rigid(spacer), // 按钮间距
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // “连接列表”按钮
				btn := material.Button(th, &st.tabList, "连接列表")
				if st.pageList { // 如果当前在列表页，高亮
					btn.Background = th.Palette.ContrastBg
				}
				if st.tabList.Clicked(gtx) {
					st.showConnListDialog = true
				}
				return btn.Layout(gtx)
			}),
			layout.Rigid(spacer), // 间距
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // “显示已删除”开关
				before := st.showDelSw.Value
				dims := material.Switch(th, &st.showDelSw, "显示已删除").Layout(gtx)
				if st.showDelSw.Value != before { // 如果开关状态改变，重新加载列表
					loadList(database, st)
				}
				return dims
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 右侧空白占位
				return layout.Dimensions{}
			}),
		)
	})
}

func sidebar(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions { // 左侧分组侧边栏布局
	inset := layout.UniformInset(unit.Dp(8)) // 四周留白
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, // 垂直排列
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 标题“连接”
				title := material.Body1(th, "连接")
				return title.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 可滚动的分组列表
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
					btn := &st.groupBtns[i] // 获取对应的点击状态
					if btn.Clicked(gtx) {   // 如果被点击
						st.currentGrp = key    // 更新当前分组
						loadList(database, st) // 刷新列表
					}
					return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions { // 绘制可点击区域
						in := layout.UniformInset(unit.Dp(4))
						return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							display := text
							if key == st.currentGrp { // 如果是当前选中项，加个点标记
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

func drawConnListDialog(gtx layout.Context, th *material.Theme, database *sql.DB, st *state) layout.Dimensions {
	max := gtx.Constraints.Max
	defer clip.Rect{Max: max}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: color.NRGBA{A: 0x80}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	dialogWidth := gtx.Dp(unit.Dp(720))
	dialogHeight := gtx.Dp(unit.Dp(420))
	size := image.Pt(dialogWidth, dialogHeight)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = size
		gtx.Constraints.Max = size
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		inset := layout.UniformInset(unit.Dp(16))
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
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th, &st.refreshBtn, "刷新")
							if st.refreshBtn.Clicked(gtx) {
								loadList(database, st)
							}
							return btn.Layout(gtx)
						}),
						layout.Rigid(spacer),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th, &st.connListDialogOK, "关闭")
							if st.connListDialogOK.Clicked(gtx) {
								st.showConnListDialog = false
							}
							return btn.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}

func openSession(th *material.Theme, st *state, c db.Connection) { // 打开或激活一个 SSH 会话
	idx := -1                    // 会话在切片中的索引，-1 表示还没找到
	for i := range st.sessions { // 遍历已有会话
		if st.sessions[i].conn.ID == c.ID { // 如果连接 ID 一致，说明该连接已在会话中
			idx = i // 记录下标
			break   // 退出循环
		}
	}
	if idx == -1 {
		st.sessions = append(st.sessions, session{
			conn:  c,
			title: strconv.Itoa(len(st.sessions)+1) + " " + c.Host,
			tag:   new(int),
		})
		st.sessionTabs = append(st.sessionTabs, widget.Clickable{}) // 为新会话增加一个 Tab 点击状态
		idx = len(st.sessions) - 1                                  // 新增会话的索引是最后一个元素
	}
	s := &st.sessions[idx]           // 取出当前会话指针
	s.logList.Axis = layout.Vertical // 会话的终端输出列表按垂直方向滚动
	if s.term == nil {               // 如果还没有建立 SSH 终端会话
		if s.log != "" { // 如果日志里已有内容（可能是之前的提示）
			s.log += "\n" // 先换行再追加新的日志
		}
		s.log += "连接主机..." // 日志中提示“连接主机...”
		// 创建 SSH 终端会话，宽 120 字符，高 32 行
		term, err := sshclient.NewTerminalSession(c.Host, c.Port, c.User, c.Password, 120, 32)
		if err != nil { // 如果连接失败
			if s.log != "" { // 如果日志非空
				s.log += "\n" // 先加一个换行
			}
			s.log += "连接失败:\n" + err.Error() // 日志中写入错误原因
			st.lastMessage = "连接失败"          // 底部状态栏也显示“连接失败”
		} else { // 连接成功
			s.term = term    // 保存终端会话对象
			if s.log != "" { // 如果日志非空
				s.log += "\n" // 先换一行
			}
			s.log += "连接主机成功"       // 日志提示连接成功
			st.lastMessage = "连接成功" // 底部状态栏显示“连接成功”
			startTerminalReader(s)  // 启动后台协程异步读取终端输出
		}
	}
	st.activeSession = idx // 把当前激活会话设为本会话
	st.focusRequested = true
}

func startTerminalReader(s *session) { // 后台读取远端终端输出，更新到 s.log
	if s == nil || s.term == nil { // 如果会话或终端为空，直接返回
		return
	}
	go func() { // 启动一个 goroutine 避免阻塞 GUI
		buf := make([]byte, 4096) // 每次读取最多 4096 字节
		for {                     // 持续循环读取
			n, err := s.term.Stdout.Read(buf) // 从远端终端的标准输出读数据
			if n > 0 {                        // 如果本次读到了数据
				chunk := make([]byte, n)                   // 新建一个切片保存实际数据
				copy(chunk, buf[:n])                       // 把 buf 中的数据拷贝出来
				appendTerminalOutput(&s.log, chunk)        // 解码并按终端控制规则更新日志
				const maxLines = 2000                      // 日志只保留 2000 行，避免内存无限增长
				if strings.Count(s.log, "\n") > maxLines { // 如果行数超过最大值
					over := strings.Count(s.log, "\n") - maxLines // 超出的行数
					pos := 0
					for i := 0; i < over && pos < len(s.log); i++ { // 从头统计需要丢弃的行
						idx := strings.IndexByte(s.log[pos:], '\n') // 找到下一个换行的位置
						if idx < 0 {                                // 找不到换行直接退出
							break
						}
						pos += idx + 1 // 跳到下一行开头
					}
					if pos > 0 && pos < len(s.log) { // 如果丢弃位置合法
						s.log = s.log[pos:] // 丢弃前面的旧日志，保留最新部分
					}
				}
				if invalidateWindow != nil { // 如果重绘函数已设置
					invalidateWindow() // 通知窗口“需要重绘”，触发终端区域刷新
				}
			}
			if err != nil { // 如果读操作出错（包括 EOF）
				break // 退出循环，结束这个协程
			}
		}
	}()
}

// appendTerminalOutput 按终端规则处理远端输出：
// - 把 \b 和 DEL(0x7f) 当作退格，从当前日志中删除最后一个字符
// - 把 \r 简单处理为换行
// - 丢弃其它不可见控制字符
func appendTerminalOutput(log *string, raw []byte) {
	text := decodeRemote(raw)
	if text == "" {
		return
	}
	*log += text
}

func sessionTabsBar(gtx layout.Context, th *material.Theme, st *state) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		func() []layout.FlexChild {
			var children []layout.FlexChild
			for i := range st.sessions {
				i := i
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := &st.sessionTabs[i]
					label := st.sessions[i].title

					if btn.Clicked(gtx) {
						// oldActive := st.activeSession
						if st.sessions[i].term == nil {
							openSession(th, st, st.sessions[i].conn)
						} else {
							st.activeSession = i
						}
						if st.activeSession >= 0 && st.activeSession < len(st.sessions) {
							// 标记需要聚焦，推迟到 sessionsArea 中执行，确保 tag 已注册
							st.focusRequested = true
							// 恢复 tag 稳定性，不再每次 new
							// st.sessions[st.activeSession].tag = new(int)
						}
						if invalidateWindow != nil {
							invalidateWindow()
						}
					}

					return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
						pad := layout.UniformInset(unit.Dp(4))
						return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dot := material.Body1(th, "●")
									if st.sessions[i].term != nil {
										dot.Color = color.NRGBA{R: 0x00, G: 0xc8, B: 0x00, A: 0xff}
									} else {
										dot.Color = color.NRGBA{R: 0xaa, G: 0xaa, B: 0xaa, A: 0xff}
									}
									return dot.Layout(gtx)
								}),
								layout.Rigid(spacer),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body1(th, label)
									if st.activeSession == i {
										lbl.Font.Weight = 600
										lbl.Color = th.Palette.ContrastBg
									}
									return lbl.Layout(gtx)
								}),
							)
						})
					})
				}))
				children = append(children, layout.Rigid(spacer))
			}
			return children
		}()...,
	)
}

func sessionsArea(gtx layout.Context, th *material.Theme, st *state) layout.Dimensions { // 下方会话终端区域
	inset := layout.UniformInset(unit.Dp(8)) // 整个区域四周留 8dp 内边距
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()                      // 限制绘制区域，避免溢出
		paint.ColorOp{Color: color.NRGBA{R: 0x12, G: 0x1b, B: 0x2b, A: 0xff}}.Add(gtx.Ops) // 设置深色背景
		paint.PaintOp{}.Add(gtx.Ops)                                                       // 实际填充背景颜色
		if st.activeSession < 0 || st.activeSession >= len(st.sessions) {
			return layout.Dimensions{}
		}
		s := &st.sessions[st.activeSession]
		if s.tag == nil {
			s.tag = new(int)
		}
		tag := s.tag
		outInset := layout.UniformInset(unit.Dp(4))
		return outInset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
			event.Op(gtx.Ops, tag)
			area.Pop()

			// 如果收到聚焦请求（来自 Tab 切换），或者是新打开的会话（term!=nil 但 tag 刚初始化），执行聚焦
			// 注意：FocusCmd 必须在 event.Op 之后执行
			if st.focusRequested {
				if guiLogger != nil {
					guiLogger.Info("DEBUG: Executing Deferred FocusCmd for Session %d, Tag %p", st.activeSession, tag)
				}
				fmt.Printf("FOCUS: Executing Deferred FocusCmd for Session %d. Tag: %p\n", st.activeSession, tag)
				gtx.Execute(key.FocusCmd{Tag: tag})
				st.focusRequested = false
			}

			// fmt.Printf("RENDER: Session %d. Tag: %p. Term: %p\n", st.activeSession, tag, s.term)

			for {
				ev, ok := gtx.Event(
					key.FocusFilter{Target: tag},
					pointer.Filter{Target: tag, Kinds: pointer.Press},
					key.Filter{Focus: tag, Name: key.NameReturn},
					key.Filter{Focus: tag, Name: key.NameEnter},
					key.Filter{Focus: tag, Name: key.NameTab},
					key.Filter{Focus: tag, Name: key.NameDeleteBackward},
					key.Filter{Focus: tag, Name: key.NameDeleteForward},
					key.Filter{Focus: tag, Name: "C", Required: key.ModShortcut | key.ModShift},
					key.Filter{Focus: tag, Name: "V", Required: key.ModShortcut | key.ModShift},
					key.Filter{Focus: tag},
					pointer.Filter{Target: tag},
				)
				if !ok {
					break
				}
				switch e := ev.(type) {
				case key.Event:
					if guiLogger != nil {
						guiLogger.Info("DEBUG: Session %d Key Event: Name=%s State=%v Tag=%p", st.activeSession, e.Name, e.State, tag)
					}
					fmt.Printf("KEY: Session %d. Name: %s. State: %v. Tag: %p\n", st.activeSession, e.Name, e.State, tag)
					if e.State != key.Press {
						continue
					}

					if (e.Modifiers & (key.ModShortcut | key.ModShift)) == (key.ModShortcut | key.ModShift) {
						nameStr := string(e.Name)
						if nameStr == "C" || nameStr == "c" {
							text := getCopyTextForSession(s)
							if text != "" {
								if guiLogger != nil {
									guiLogger.Info("GUI Copy terminal selectedText len=%d", len(text))
								}
								if err := clipboard.WriteAll(text); err != nil {
									s.log += "\n错误:\n" + err.Error()
									if invalidateWindow != nil {
										invalidateWindow()
									}
									st.lastMessage = "复制到剪贴板失败"
								} else {
									st.lastMessage = "已复制选中内容到剪贴板"
								}
							}
							continue
						}
						if nameStr == "V" || nameStr == "v" {
							if s.term == nil || s.term.Stdin == nil {
								st.lastMessage = "当前会话未连接，无法粘贴"
								continue
							}
							text, err := clipboard.ReadAll()
							if err != nil {
								s.log += "\n错误:\n" + err.Error()
								if invalidateWindow != nil {
									invalidateWindow()
								}
								st.lastMessage = "读取剪贴板失败"
								continue
							}
							if text != "" {
								if _, err := s.term.Stdin.Write([]byte(text)); err != nil {
									s.log += "\n错误:\n" + err.Error()
									if invalidateWindow != nil {
										invalidateWindow()
									}
									st.lastMessage = "粘贴失败"
								}
							}
							continue
						}
					}

					if s.term == nil || s.term.Stdin == nil {
						continue
					}

					nameStr := string(e.Name)
					var r rune
					if len(nameStr) > 0 {
						r, _ = utf8.DecodeRuneInString(nameStr)
					}
					var data string
					switch e.Name {
					case key.NameReturn, key.NameEnter:
						data = "\n"
					case key.NameTab:
						data = "\t"
					case key.NameDeleteBackward:
						data = "\x7f"
					case key.NameDeleteForward:
						data = "\x1b[3~"
					case key.NameLeftArrow:
						data = "\x1b[D"
					case key.NameRightArrow:
						data = "\x1b[C"
					case key.NameUpArrow:
						data = "\x1b[A"
					case key.NameDownArrow:
						data = "\x1b[B"
					case key.NameHome:
						data = "\x1b[H"
					case key.NameEnd:
						data = "\x1b[F"
					case key.NamePageUp:
						data = "\x1b[5~"
					case key.NamePageDown:
						data = "\x1b[6~"
					case key.NameEscape:
						data = "\x1b"
					default:
						data = ""
					}

					if data == "" && len(nameStr) > 0 {
						switch r {
						case '\b', 0x7f, 0x232B:
							data = "\x7f"
						case 0x2326:
							data = "\x1b[3~"
						}
					}

					if guiLogger != nil && (e.Name == key.NameDeleteBackward || e.Name == key.NameDeleteForward ||
						r == '\b' || r == 0x7f || r == 0x232B || r == 0x2326) {
						guiLogger.Info("GUI KeyEvent delete name=%q rune=%U data_hex=%x", nameStr, r, []byte(data))
					}

					if data != "" {
						fmt.Printf("WRITE: Session %d. Writing %q to Stdin %p\n", st.activeSession, data, s.term.Stdin)
						_, err := s.term.Stdin.Write([]byte(data))
						if err != nil {
							fmt.Printf("WRITE ERROR: Session %d. Error: %v\n", st.activeSession, err)
							s.log += "\n错误:\n" + err.Error()
							if invalidateWindow != nil {
								invalidateWindow()
							}
							st.lastMessage = "命令发送失败"
						}
					}
				case key.EditEvent:
					if s.term == nil || s.term.Stdin == nil {
						fmt.Printf("EDIT: Session %d. Stdin is nil\n", st.activeSession)
						continue
					}
					if e.Text != "" {
						fmt.Printf("EDIT: Session %d. Writing %q to Stdin %p\n", st.activeSession, e.Text, s.term.Stdin)
						_, err := s.term.Stdin.Write([]byte(e.Text))
						if err != nil {
							fmt.Printf("EDIT ERROR: Session %d. Error: %v\n", st.activeSession, err)
							s.log += "\n错误:\n" + err.Error()
							if invalidateWindow != nil {
								invalidateWindow()
							}
							st.lastMessage = "命令发送失败"
						}
						continue
					}

					if e.Range.End > e.Range.Start {
						count := e.Range.End - e.Range.Start

						if count > 0 {
							buf := make([]byte, count)
							for i := 0; i < count; i++ {
								buf[i] = 0x7f
							}
							if guiLogger != nil {
								guiLogger.Info("GUI EditEvent delete count=%d data_hex=%x", count, buf)
							}
							_, err := s.term.Stdin.Write(buf)
							if err != nil {
								s.log += "\n错误:\n" + err.Error()
								if invalidateWindow != nil {
									invalidateWindow()
								}
								st.lastMessage = "命令发送失败"
							}
						}
					}
				case pointer.Event:
					if e.Kind == pointer.Press {
						if guiLogger != nil {
							guiLogger.Info("DEBUG: Pointer Clicked Session %d, Tag %p", st.activeSession, tag)
						}
						fmt.Printf("CLICK: Session %d. Tag: %p\n", st.activeSession, tag)
						gtx.Execute(key.FocusCmd{Tag: tag})
						if e.Buttons == pointer.ButtonSecondary {
							s.contextMenuOpen = true
						} else {
							s.contextMenuOpen = false
						}
						if invalidateWindow != nil {
							invalidateWindow()
						}
					}
				default:
					continue
				}
			}

			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layoutAnsiText(gtx, th, &s.logList, s.log, true)
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if !s.contextMenuOpen || s.log == "" {
						return layout.Dimensions{}
					}
					return drawSelectionToolbar(gtx, th, s, st)
				}),
			)
		})
	})
}

func getCopyTextForSession(s *session) string {
	if s == nil {
		return ""
	}
	if s.log == "" {
		return ""
	}
	return ansiToPlain(s.log)
}

type ansiRun struct { // 一段连续文本及其对应的前景色
	text  string      // 文本内容
	color color.NRGBA // 文本颜色
}

func layoutAnsiText(gtx layout.Context, th *material.Theme, list *layout.List, s string, showCursor bool) layout.Dimensions {
	lines := strings.Split(s, "\n") // 把整段日志按行拆分
	list.Axis = layout.Vertical     // 列表垂直方向滚动
	list.ScrollToEnd = true         // 自动滚动到最后一行，显示最新输出
	return list.Layout(gtx, len(lines), func(gtx layout.Context, i int) layout.Dimensions {
		line := lines[i]                // 当前第 i 行字符串
		isLastLine := i == len(lines)-1 // 是否最后一行
		runs := parseANSIRuns(line)     // 解析这一行里的 ANSI 转义，得到带颜色的片段
		if isLastLine && showCursor {   // 如果是最后一行且需要显示光标
			runs = append(runs, ansiRun{ // 在末尾追加一个“█”块表示光标
				text:  "█",
				color: color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff},
			})
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, // 一行内横向排布所有片段
			func() []layout.FlexChild {
				var rs []layout.FlexChild
				for _, r := range runs { // 遍历所有颜色片段
					r := r // 防止闭包捕获同一个变量
					rs = append(rs, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(th, r.text) // 用 Body1 样式展示这一段文字
						lbl.Color = r.color               // 设置该段文字颜色
						return lbl.Layout(gtx)            // 绘制文字
					}))
				}
				if len(rs) == 0 { // 如果这一行是完全空行
					rs = append(rs, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{} // 返回空尺寸占位
					}))
				}
				return rs // 返回该行所有子元素
			}()...,
		)
	})
}

func ansiToPlain(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, line := range lines {
		runs := parseANSIRuns(line)
		for _, r := range runs {
			b.WriteString(r.text)
		}
		if i != len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func drawSelectionToolbar(gtx layout.Context, th *material.Theme, s *session, st *state) layout.Dimensions {
	content := func(gtx layout.Context) layout.Dimensions {
		inset := layout.UniformInset(unit.Dp(4))
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.menuCopyBtn.Clicked(gtx) {
						text := getCopyTextForSession(s)
						if text != "" {
							if guiLogger != nil {
								guiLogger.Info("GUI Copy terminal selectedText len=%d", len(text))
							}
							if err := clipboard.WriteAll(text); err != nil {
								s.log += "\n错误:\n" + err.Error()
								if invalidateWindow != nil {
									invalidateWindow()
								}
							} else {
								st.lastMessage = "已复制选中内容到剪贴板"
							}
						}
						s.contextMenuOpen = false
					}
					return material.Clickable(gtx, &s.menuCopyBtn, func(gtx layout.Context) layout.Dimensions {
						rowPad := layout.UniformInset(unit.Dp(4))
						return rowPad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body1(th, "复制")
									return lbl.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{}
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body1(th, "Ctrl+Shift+C")
									return lbl.Layout(gtx)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.menuPasteBtn.Clicked(gtx) {
						if s.term == nil || s.term.Stdin == nil {
							st.lastMessage = "当前会话未连接，无法粘贴"
						} else {
							text, err := clipboard.ReadAll()
							if err != nil {
								s.log += "\n错误:\n" + err.Error()
								if invalidateWindow != nil {
									invalidateWindow()
								}
								st.lastMessage = "读取剪贴板失败"
							} else if text != "" {
								if _, err := s.term.Stdin.Write([]byte(text)); err != nil {
									s.log += "\n错误:\n" + err.Error()
									if invalidateWindow != nil {
										invalidateWindow()
									}
									st.lastMessage = "粘贴失败"
								}
							}
						}
						s.contextMenuOpen = false
					}
					return material.Clickable(gtx, &s.menuPasteBtn, func(gtx layout.Context) layout.Dimensions {
						rowPad := layout.UniformInset(unit.Dp(4))
						return rowPad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body1(th, "粘贴")
									return lbl.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{}
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body1(th, "Ctrl+Shift+V")
									return lbl.Layout(gtx)
								}),
							)
						})
					})
				}),
			)
		})
	}

	// 先记录内容以便获取尺寸
	macro := op.Record(gtx.Ops)
	dims := content(gtx)
	call := macro.Stop()

	// 估算单个字符宽高
	charMacro := op.Record(gtx.Ops)
	sampleLbl := material.Body1(th, "W")
	charDims := sampleLbl.Layout(gtx)
	charCall := charMacro.Stop()
	_ = charCall

	charW := charDims.Size.X
	charH := charDims.Size.Y
	if charW <= 0 {
		charW = gtx.Dp(unit.Dp(8))
	}
	if charH <= 0 {
		charH = gtx.Dp(unit.Dp(16))
	}

	// 计算最后一行文本长度，忽略 ANSI 颜色码
	lines := strings.Split(s.log, "\n")
	last := ""
	if len(lines) > 0 {
		last = lines[len(lines)-1]
	}
	runs := parseANSIRuns(last)
	colRunes := 0
	for _, r := range runs {
		colRunes += utf8.RuneCountInString(r.text)
	}

	max := gtx.Constraints.Max
	margin := gtx.Dp(unit.Dp(4))
	cursorX := colRunes*charW + margin
	cursorY := max.Y - charH - dims.Size.Y - margin
	if cursorY < 0 {
		cursorY = 0
	}
	if cursorX+dims.Size.X > max.X-margin {
		cursorX = max.X - dims.Size.X - margin
	}
	if cursorX < 0 {
		cursorX = 0
	}

	offset := op.Offset(image.Pt(cursorX, cursorY)).Push(gtx.Ops)

	rect := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()

	border := widget.Border{
		Color:        color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff},
		CornerRadius: unit.Dp(6),
		Width:        unit.Dp(1),
	}
	res := border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		call.Add(gtx.Ops)
		return dims
	})
	offset.Pop()
	return res
}

func parseANSIRuns(s string) []ansiRun { // 解析一行字符串中的 ANSI 颜色控制，拆成多段带颜色文本
	defColor := color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff} // 默认前景色（浅灰）
	curColor := defColor                                        // 当前生效的颜色
	isBold := false                                             // 当前是否处于“加粗”状态
	var runs []ansiRun                                          // 解析结果：多段文本及其颜色
	var buf []rune                                              // 临时缓冲当前片段的字符

	flush := func() { // 把 buf 中累积的字符作为一个 ansiRun 输出
		if len(buf) == 0 { // 如果缓冲为空，则不输出
			return
		}
		runs = append(runs, ansiRun{ // 追加一段新片段
			text:  string(buf), // 文本内容为当前缓冲
			color: curColor,    // 使用当前颜色
		})
		buf = buf[:0] // 清空缓冲，为下一段做准备
	}

	i := 0 // 当前扫描的字节索引
OuterLoop:
	for i < len(s) { // 从头到尾扫描整个字符串
		if s[i] == 0x1b { // 0x1b = ESC，表示 ANSI 序列的开始
			// CSI: ESC [ ...
			if i+1 < len(s) && s[i+1] == '[' { // ESC 后面跟 '['，表示 CSI 控制序列
				j := i + 2 // 从 '[' 后面的第一个字符开始扫描参数
				for j < len(s) {
					c := s[j]
					if c >= 0x40 && c <= 0x7E { // 0x40~0x7E 的字符表示 CSI 序列结束符
						if c == 'm' { // 'm' 结尾表示 SGR（图形属性：颜色/加粗）
							flush()                                // 应用新颜色之前，先输出旧颜色的片段
							params := strings.Split(s[i+2:j], ";") // 取 ESC[ 和 'm' 之间的参数列表
							var codes []int
							for _, p := range params { // 把每个参数由字符串转为整数
								if p != "" {
									v, _ := strconv.Atoi(p)
									codes = append(codes, v)
								}
							}
							if len(codes) == 0 { // 没有参数时等价于 0（重置所有属性）
								codes = []int{0}
							}

							k := 0
							for k < len(codes) { // 遍历所有 SGR 参数
								v := codes[k]
								switch {
								case v == 0: // 0 = 重置颜色和加粗等所有属性
									curColor = defColor
									isBold = false
								case v == 1: // 1 = 打开加粗
									isBold = true
								case 30 <= v && v <= 37: // 30~37 = 基本前景色（8 色）
									curColor = mapSGRColor(v, isBold)
								case 90 <= v && v <= 97: // 90~97 = 亮色前景色
									curColor = mapSGRColor(v-60, true)
								case v == 39: // 39 = 恢复默认前景色
									curColor = defColor
									isBold = false
								case v == 38: // 38;5;n = 256 色前景色
									// 38;5;n
									if k+2 < len(codes) && codes[k+1] == 5 {
										curColor = map256Color(codes[k+2]) // 根据 n 映射为具体颜色
										k += 2                             // 跳过后面两个参数（5 和 n）
									}
								}
								k++
							}
						}
						i = j + 1          // 把 i 移到整个 CSI 序列之后
						continue OuterLoop // 回到外层循环，继续处理后面的字符
					}
					j++ // 还没遇到结束符，继续往后扫描
				}
				// 如果走到这里说明 CSI 序列不完整或损坏，直接跳出 CSI 处理逻辑
				break
			}
			// OSC: ESC ] ... BEL or ST （操作系统控制序列，例如设置标题）
			if i+1 < len(s) && s[i+1] == ']' {
				j := i + 2
				for j < len(s) {
					if s[j] == 0x07 { // BEL（\a）表示 OSC 结束
						i = j + 1
						continue OuterLoop
					}
					if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' { // ESC '\' 表示 ST 结束
						i = j + 2
						continue OuterLoop
					}
					j++
				}
				break // 不完整的 OSC 序列，跳出处理
			}
			// 其它 ESC 序列：直接跳过 ESC 字节，避免在界面上显示奇怪符号
			i++
			continue
		}

		// 普通字符路径：先按 UTF-8 解码出一个 rune
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError { // 解码失败，跳过一个字节
			i++
			continue
		}
		// 过滤不可见控制字符（ASCII < 32），但保留 '\t' 和 '\n'
		if r < 32 && r != '\t' && r != '\n' {
			i += size
			continue
		}

		if r == '\t' { // Tab 转为四个空格，方便等宽显示
			buf = append(buf, ' ', ' ', ' ', ' ')
		} else {
			buf = append(buf, r) // 普通字符直接加入当前片段缓冲
		}
		i += size // 前进到下一个 rune
	}
	flush()     // 处理完后，把最后一段缓冲输出为 ansiRun
	return runs // 返回整行被切分好的彩色片段列表
}

func mapSGRColor(code int, bold bool) color.NRGBA {
	if bold {
		switch code {
		case 30:
			return color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}
		case 31:
			return color.NRGBA{R: 0xff, G: 0x55, B: 0x55, A: 0xff}
		case 32:
			return color.NRGBA{R: 0x55, G: 0xff, B: 0x55, A: 0xff}
		case 33:
			return color.NRGBA{R: 0xff, G: 0xff, B: 0x55, A: 0xff}
		case 34:
			return color.NRGBA{R: 0x55, G: 0x55, B: 0xff, A: 0xff}
		case 35:
			return color.NRGBA{R: 0xff, G: 0x55, B: 0xff, A: 0xff}
		case 36:
			return color.NRGBA{R: 0x55, G: 0xff, B: 0xff, A: 0xff}
		case 37:
			return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		}
	} else {
		switch code {
		case 30:
			return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
		case 31:
			return color.NRGBA{R: 0xcd, G: 0x00, B: 0x00, A: 0xff}
		case 32:
			return color.NRGBA{R: 0x00, G: 0xcd, B: 0x00, A: 0xff}
		case 33:
			return color.NRGBA{R: 0xcd, G: 0xcd, B: 0x00, A: 0xff}
		case 34:
			return color.NRGBA{R: 0x00, G: 0x00, B: 0xee, A: 0xff}
		case 35:
			return color.NRGBA{R: 0xcd, G: 0x00, B: 0xcd, A: 0xff}
		case 36:
			return color.NRGBA{R: 0x00, G: 0xcd, B: 0xcd, A: 0xff}
		case 37:
			return color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff}
		}
	}
	return color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
}

func map256Color(code int) color.NRGBA {
	if code < 16 {
		bold := code >= 8
		c := code
		if bold {
			c -= 8
		}
		return mapSGRColor(30+c, bold)
	}
	if code < 232 {
		c := code - 16
		r := (c / 36) * 51
		g := ((c / 6) % 6) * 51
		b := (c % 6) * 51
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
	}
	c := code - 232
	v := 8 + c*10
	return color.NRGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: 0xff}
}

func decodeRemote(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	return string(b)
}
