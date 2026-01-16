package gui // 本文件所在的包名，GUI 相关代码都在这个包里

import (
	"database/sql" // 标准库：操作数据库连接（*sql.DB）
	"image/color"  // 标准库：颜色结构（NRGBA 等），用于界面颜色
	"sort"         // 标准库：排序工具（用于分组排序）
	"strconv"      // 标准库：字符串和数字之间转换
	"strings"      // 标准库：字符串处理（分割、替换等）
	"time"         // 标准库：时间和日期
	"unicode/utf8" // 标准库：UTF-8 字符串解码（逐个 rune 解析）

	"gioui.org/app"                                // Gio：应用和窗口管理（创建窗口、事件循环）
	"gioui.org/io/event"                           // Gio：通用事件系统（键盘、鼠标等 Tag）
	"gioui.org/io/key"                             // Gio：键盘事件和输入法事件（KeyEvent、EditEvent）
	"gioui.org/layout"                             // Gio：通用布局（Flex、List、Inset 等）
	"gioui.org/op"                                 // Gio：绘制操作容器
	"gioui.org/op/clip"                            // Gio：裁剪绘制区域（clip.Rect）
	"gioui.org/op/paint"                           // Gio：填充颜色、绘制操作
	"gioui.org/unit"                               // Gio：dp / sp 等单位转换
	"gioui.org/widget"                             // Gio：基础控件状态（Editor、List、Clickable 等）
	"gioui.org/widget/material"                    // Gio：Material Design 风格的组件绘制
	"golang.org/x/text/encoding/simplifiedchinese" // 第三方：简体中文编码（GBK/GB18030）转换
	"ssh-go/internal/db"                           // 本项目：数据库访问（连接配置增删改查）
	"ssh-go/internal/logging"                      // 本项目：简单日志封装
	"ssh-go/internal/sshclient"                    // 本项目：SSH 客户端与终端会话封装
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
}

type session struct { // 单个 SSH 会话（终端标签页）的状态
	conn    db.Connection              // 该会话对应的连接配置（IP、端口、用户等）
	title   string                     // 会话标题，在顶部 Tab 上显示
	log     string                     // 从远端读取到的终端文本日志（包括 ANSI 转义）
	cmdEd   widget.Editor              // 预留的命令输入编辑器（当前未真正使用）
	term    *sshclient.TerminalSession // SSH 终端会话对象，封装了 Stdin/Stdout
	logList layout.List                // 展示 log 的滚动列表状态，用于自动滚动到底部
}

var invalidateWindow func() // 保存一个函数，用于在其他 goroutine 中请求窗口重绘

func Start(database *sql.DB, logger *logging.Logger) error { // GUI 程序入口，由 main 调用
	go func() { // 启动一个 goroutine 运行窗口事件循环
		w := new(app.Window)                         // 创建一个新的 Gio 窗口
		invalidateWindow = func() { w.Invalidate() } // 记录重绘函数，供其他地方调用
		w.Option(                                    // 设置窗口的一些属性
			app.Size(unit.Dp(900), unit.Dp(600)), // 窗口尺寸为 900x600 dp
			app.Title("连接管理"),                    // 窗口标题文字
		)
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
				gtx := app.NewContext(&ops, e)                 // 构造布局上下文，用于布局和绘制
				layout.Flex{Axis: layout.Vertical}.Layout(gtx, // 垂直方向整体布局：上中下
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 顶部工具栏区域
						return topBar(gtx, th, database, &st) // 绘制“添加连接/连接列表/显示已删除”
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 中间主内容区域
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, // 水平：左侧分组 + 右侧内容
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 左侧固定宽度侧边栏
								width := gtx.Dp(unit.Dp(220))          // 侧边栏宽度 220dp
								gtx.Constraints.Min.X = width          // 最小宽度设为 220
								gtx.Constraints.Max.X = width          // 最大宽度也设为 220（固定宽）
								return sidebar(gtx, th, database, &st) // 绘制分组列表
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 右侧弹性区域
								if st.pageList { // 如果当前为“连接列表”页
									return listPage(gtx, th, database, &st) // 显示连接列表+终端区域
								}
								return addPage(gtx, th, database, &st) // 否则显示“添加连接”页面
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 底部状态栏区域
						if st.lastMessage == "" { // 如果没有消息要显示
							return layout.Dimensions{} // 返回空尺寸，不占空间
						}
						lbl := material.Body1(th, st.lastMessage) // 用 Body1 样式显示状态文字
						return lbl.Layout(gtx)                    // 绘制状态文字
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
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 上半部分：连接列表，占用大部分空间
				l := material.List(th, &st.connList) // 使用可滚动 List 展示所有连接
				return l.Layout(gtx, len(st.items), func(gtx layout.Context, i int) layout.Dimensions {
					it := st.items[i]                                       // 当前第 i 条连接
					connect := &st.connectBtns[i]                           // “连接”按钮状态
					remove := &st.removeBtns[i]                             // “删除/恢复”按钮状态
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, // 一条记录在一行里横向排布
						layout.Rigid(material.Body1(th, it.Name).Layout), // 名称
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.Host).Layout), // 主机
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.Itoa(it.Port)).Layout), // 端口
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.User).Layout), // 用户
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, it.GroupName).Layout), // 分组
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.Itoa(it.Enabled)).Layout), // 是否启用（0/1）
						layout.Rigid(spacer),
						layout.Rigid(material.Body1(th, strconv.FormatInt(it.CreatedAt, 10)).Layout), // 创建时间（时间戳）
						layout.Rigid(spacer),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { // “连接”按钮
							btn := material.Button(th, connect, "连接")
							if connect.Clicked(gtx) { // 点击后打开或激活对应 SSH 会话
								openSession(th, st, it)
							}
							return btn.Layout(gtx)
						}),
						layout.Rigid(spacer),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { // “删除/恢复”按钮
							text := "删除"
							if it.Deleted == 1 { // 已删除的显示为“恢复”
								text = "恢复"
							}
							btn := material.Button(th, remove, text)
							if remove.Clicked(gtx) {
								d := 1
								if it.Deleted == 1 { // 如果当前是已删除，则点击后恢复
									d = 0
								}
								if err := db.SetDeleted(database, it.ID, d); err != nil { // 更新 Deleted 标记
									st.lastMessage = err.Error()
								} else {
									loadList(database, st) // 更新成功后重新加载列表
								}
							}
							return btn.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 中间：会话和终端区域
				if len(st.sessions) == 0 { // 没有打开任何会话则不显示
					return layout.Dimensions{}
				}
				return sessionsArea(gtx, th, st)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 底部：刷新按钮
				btn := material.Button(th, &st.refreshBtn, "刷新")
				if st.refreshBtn.Clicked(gtx) {
					loadList(database, st) // 手动重新加载列表
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

func openSession(th *material.Theme, st *state, c db.Connection) { // 打开或激活一个 SSH 会话
	idx := -1                    // 会话在切片中的索引，-1 表示还没找到
	for i := range st.sessions { // 遍历已有会话
		if st.sessions[i].conn.ID == c.ID { // 如果连接 ID 一致，说明该连接已在会话中
			idx = i // 记录下标
			break   // 退出循环
		}
	}
	if idx == -1 { // 如果没找到，说明是新会话
		st.sessions = append(st.sessions, session{ // 在 sessions 切片末尾追加一个 session
			conn:  c,                                               // 保存连接配置
			title: strconv.Itoa(len(st.sessions)+1) + " " + c.Host, // 标题 = 序号+主机
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
				s.log += decodeRemote(chunk)               // 解码远端编码（UTF-8 / GBK）追加到日志
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

func sessionsArea(gtx layout.Context, th *material.Theme, st *state) layout.Dimensions { // 下方会话列表+终端区域
	inset := layout.UniformInset(unit.Dp(8)) // 整个区域四周留 8dp 内边距
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()                      // 限制绘制区域，避免溢出
		paint.ColorOp{Color: color.NRGBA{R: 0x12, G: 0x1b, B: 0x2b, A: 0xff}}.Add(gtx.Ops) // 设置深色背景
		paint.PaintOp{}.Add(gtx.Ops)                                                       // 实际填充背景颜色
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,                              // 垂直方向：上 Tab，下面终端内容
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { // 上方会话 Tab 区域
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, // 水平方向排列多个 Tab
					func() []layout.FlexChild {
						var children []layout.FlexChild // 存放所有 Tab 的子项
						for i := range st.sessions {    // 为每个会话生成一个 Tab
							i := i // 防止闭包捕获同一个 i
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := &st.sessionTabs[i]     // 对应第 i 个会话的 Tab 按钮
								label := st.sessions[i].title // Tab 显示的标题文字
								return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
									if btn.Clicked(gtx) { // 如果用户点击了该 Tab
										st.activeSession = i // 切换当前激活会话
									}
									pad := layout.UniformInset(unit.Dp(4)) // Tab 内部四周 4dp 内边距
									return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body1(th, label) // Tab 内部的文字标签
										if st.activeSession == i {       // 如果是当前激活 Tab
											lbl.Font.Weight = 600 // 字体加粗显示
										}
										return lbl.Layout(gtx) // 绘制 Tab 文本
									})
								})
							}))
							children = append(children, layout.Rigid(spacer)) // 每个 Tab 之间插入一个空隙
						}
						return children // 返回所有 Tab 组成的列表
					}()...,
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { // 下方终端内容区域（占用剩余空间）
				if st.activeSession < 0 || st.activeSession >= len(st.sessions) { // 没有激活会话
					return layout.Dimensions{} // 不绘制任何内容
				}
				s := &st.sessions[st.activeSession]         // 取当前激活会话
				outInset := layout.UniformInset(unit.Dp(4)) // 终端内容四周再留 4dp 内边距
				return outInset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					// 把当前会话 s 注册为可接收键盘事件的目标
					// 这样键盘输入（回车、Tab、方向键等）就会发给该会话，而不是其它控件
					event.Op(gtx.Ops, s)              // 注册事件 Tag
					gtx.Execute(key.FocusCmd{Tag: s}) // 请求把键盘焦点设置到当前会话

					for { // 循环处理本帧中所有与 s 相关的键盘事件
						ev, ok := gtx.Event(
							key.FocusFilter{Target: s},                         // 获取与 s 相关的焦点/输入法事件
							key.Filter{Focus: s, Name: key.NameReturn},         // 过滤回车键（Return）
							key.Filter{Focus: s, Name: key.NameEnter},          // 过滤回车键（Enter）
							key.Filter{Focus: s, Name: key.NameTab},            // 过滤 Tab 键
							key.Filter{Focus: s, Name: key.NameDeleteBackward}, // 过滤退格键 Backspace
							key.Filter{Focus: s, Name: key.NameDeleteForward},  // 过滤 Delete 键
							key.Filter{Focus: s},                               // 其它普通键
						)
						if !ok { // 如果没有事件可以处理了
							break // 退出事件处理循环
						}
						switch e := ev.(type) { // 根据事件类型分支
						case key.Event: // 单个键按下/松开事件
							if e.State != key.Press { // 只关心“按下”，忽略“松开”
								continue
							}
							if s.term == nil || s.term.Stdin == nil { // 没有终端会话则不处理
								continue
							}
							var data string // 要发送到远端的控制序列字符串
							switch e.Name { // 根据按键名称决定发送什么
							case key.NameReturn, key.NameEnter: // 回车键
								data = "\n" // 发送换行符
							case key.NameTab: // Tab 键
								data = "\t" // 发送制表符
							case key.NameDeleteBackward: // 退格键（Backspace）
								data = "\x7f" // 发送 DEL(0x7f)，一般配置下表示退格
							case key.NameDeleteForward: // Delete 键
								data = "\x1b[3~" // ANSI 序列：Delete
							case key.NameLeftArrow: // 左方向键
								data = "\x1b[D"
							case key.NameRightArrow: // 右方向键
								data = "\x1b[C"
							case key.NameUpArrow: // 上方向键
								data = "\x1b[A"
							case key.NameDownArrow: // 下方向键
								data = "\x1b[B"
							case key.NameHome: // Home 键
								data = "\x1b[H"
							case key.NameEnd: // End 键
								data = "\x1b[F"
							case key.NamePageUp: // PageUp 键
								data = "\x1b[5~"
							case key.NamePageDown: // PageDown 键
								data = "\x1b[6~"
							case key.NameEscape: // ESC 键
								data = "\x1b"
							default:
								data = "" // 其它特殊键暂时不处理
							}
							if data != "" { // 如果有数据要发
								_, err := s.term.Stdin.Write([]byte(data)) // 写入远端终端的标准输入
								if err != nil {                            // 写入失败
									s.log += "\n错误:\n" + err.Error() // 在终端日志里追加错误信息
									if invalidateWindow != nil {     // 请求窗口重绘
										invalidateWindow()
									}
									st.lastMessage = "命令发送失败" // 底部状态栏提示失败
								}
							}
						case key.EditEvent: // 输入法编辑事件（文本输入或删除）
							if s.term == nil || s.term.Stdin == nil { // 没有终端会话则忽略
								continue
							}
							if e.Text != "" { // Text 非空：表示有文本要插入
								_, err := s.term.Stdin.Write([]byte(e.Text)) // 直接把文字写到远端
								if err != nil {
									s.log += "\n错误:\n" + err.Error()
									if invalidateWindow != nil {
										invalidateWindow()
									}
									st.lastMessage = "命令发送失败"
								}
								continue // 该事件已处理完，继续处理下一个
							}
							// Text 为空，但 Range 有长度：通常表示删除操作
							if e.Range.End > e.Range.Start {
								count := e.Range.End - e.Range.Start // 需要删除的字符数
								if count > 0 {
									buf := make([]byte, count) // 构造多个退格控制码
									for i := 0; i < count; i++ {
										buf[i] = 0x7f // 每个字符用 DEL(0x7f) 表示删除
									}
									_, err := s.term.Stdin.Write(buf) // 发送删除控制码到远端
									if err != nil {
										s.log += "\n错误:\n" + err.Error()
										if invalidateWindow != nil {
											invalidateWindow()
										}
										st.lastMessage = "命令发送失败"
									}
								}
							}
						default:
							continue // 其它类型事件暂时不处理
						}
					}
					// 处理完所有键盘事件后，渲染终端文本区域（带滚动和光标）
					return layoutAnsiText(gtx, th, &s.logList, s.log, true)
				})
			}),
		)
	})
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
