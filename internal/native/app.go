package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"node-control/internal/client"
)

type NativeApp struct {
	app         *tview.Application
	pages       *tview.Pages
	store       *client.ServerStore
	api         *AgentAPI
	activeTheme ThemeName
	currentPath string

	// Main Layout
	headerBar *tview.TextView
	statusBar *tview.TextView
	mainView  *tview.Pages

	// Views
	dashboardFlex *tview.Flex
	filesFlex     *tview.Flex
	dbFlex        *tview.Flex
	scriptsFlex   *tview.Flex

	// Files components
	filesTable   *tview.Table
	filesPathLbl *tview.InputField

	// Scripts components
	scriptIdInput  *tview.InputField
	scriptLangDrop *tview.DropDown
	scriptEditor   *tview.TextArea
	scriptLogsView *tview.TextView
	scriptRunBtn   *tview.Button
	isScriptRun    bool

	// DB components
	dbTable     *tview.Table
	dbQueryArea *tview.TextArea
	dbResTable  *tview.Table
	kvKeyInput  *tview.InputField
	kvValInput  *tview.TextArea
}

func NewNativeApp(store *client.ServerStore) *NativeApp {
	app := tview.NewApplication()
	api := NewAgentAPI(store)

	na := &NativeApp{
		app:         app,
		pages:       tview.NewPages(),
		store:       store,
		api:         api,
		activeTheme: ThemeDark,
		currentPath: "",
	}

	ApplyTheme(na.activeTheme)
	na.setupUI()
	return na
}

func (na *NativeApp) Run() error {
	na.app.EnableMouse(true)
	na.startBackgroundRefresh()
	return na.app.SetRoot(na.pages, true).Run()
}

func (na *NativeApp) setupUI() {
	// Top Header
	na.headerBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	na.updateHeader()

	// Bottom Status Bar
	na.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	na.updateStatusBar()

	// Main View Pages (Dashboard, Files, DB, Scripts)
	na.mainView = tview.NewPages()
	na.buildDashboardView()
	na.buildFilesView()
	na.buildDBView()
	na.buildScriptsView()

	na.mainView.AddPage("dashboard", na.dashboardFlex, true, true)
	na.mainView.AddPage("files", na.filesFlex, true, false)
	na.mainView.AddPage("db", na.dbFlex, true, false)
	na.mainView.AddPage("scripts", na.scriptsFlex, true, false)

	// Layout Frame
	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(na.headerBar, 2, 1, false).
		AddItem(na.mainView, 0, 1, true).
		AddItem(na.statusBar, 1, 1, false)

	na.pages.AddPage("main", mainLayout, true, true)

	// Global Keybindings
	na.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if na.pages.HasPage("modal") {
			if event.Key() == tcell.KeyEscape {
				na.pages.RemovePage("modal")
				return nil
			}
			return event
		}

		switch event.Key() {
		case tcell.KeyF1:
			na.switchView("dashboard")
			return nil
		case tcell.KeyF2:
			na.switchView("files")
			return nil
		case tcell.KeyF3:
			na.switchView("db")
			return nil
		case tcell.KeyF4:
			na.switchView("scripts")
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '1':
				if !na.isInputFocused() {
					na.switchView("dashboard")
					return nil
				}
			case '2':
				if !na.isInputFocused() {
					na.switchView("files")
					return nil
				}
			case '3':
				if !na.isInputFocused() {
					na.switchView("db")
					return nil
				}
			case '4':
				if !na.isInputFocused() {
					na.switchView("scripts")
					return nil
				}
			case 's', 'S', 'ы', 'Ы':
				if !na.isInputFocused() {
					na.showServerModal()
					return nil
				}
			case 't', 'T', 'е', 'Е':
				if !na.isInputFocused() {
					na.showThemeModal()
					return nil
				}
			case 'q', 'Q', 'й', 'Й':
				if !na.isInputFocused() {
					na.app.Stop()
					return nil
				}
			}
		}
		return event
	})
}

func (na *NativeApp) isInputFocused() bool {
	f := na.app.GetFocus()
	if f == nil {
		return false
	}
	switch f.(type) {
	case *tview.InputField, *tview.TextArea:
		return true
	default:
		return false
	}
}

func (na *NativeApp) switchView(viewName string) {
	na.mainView.SwitchToPage(viewName)
	switch viewName {
	case "dashboard":
		na.refreshDashboard()
	case "files":
		na.refreshFiles()
	case "db":
		na.refreshDB()
	case "scripts":
		na.refreshScripts()
	}
	na.updateHeader()
}

func (na *NativeApp) updateHeader() {
	activeSrv, err := na.store.GetActive()
	srvInfo := "[red]● Нет активного узла[-]"
	if err == nil && activeSrv != nil {
		dot := "[green]●"
		if activeSrv.Status == "offline" {
			dot = "[red]●"
		}
		srvInfo = fmt.Sprintf("%s [bold]%s[-] [gray](%s)[-] [aqua]%dms[-]", dot, activeSrv.Name, activeSrv.URL, activeSrv.PingMs)
	}

	themeTitle := Palettes[na.activeTheme].Title
	na.headerBar.SetText(fmt.Sprintf(" [yellow:b]Node Control[-]  [gray]|[-]  Сервер: %s  [gray]|[-]  Тема: [purple]%s[-]\n [gray]Переключение разделов: [yellow][1/F1][-] Обзор  [yellow][2/F2][-] Файлы  [yellow][3/F3][-] Базы  [yellow][4/F4][-] Скрипты  |  [yellow][S][-] Серверы  [yellow][T][-] Тема  [yellow][Q][-] Выход[-]", srvInfo, themeTitle))
}

func (na *NativeApp) updateStatusBar() {
	na.statusBar.SetText(" [yellow]Tab/Shift+Tab[-] Навигация  •  [yellow]Enter[-] Выбрать  •  [yellow]Esc[-] Назад  •  [aqua]Мышь поддерживается[-]")
}

func (na *NativeApp) startBackgroundRefresh() {
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			activeSrv, err := na.store.GetActive()
			if err == nil && activeSrv != nil {
				_, ping, hErr := na.api.GetHealth()
				if hErr != nil {
					na.store.UpdateStatus(activeSrv.ID, "offline", 0)
				} else {
					na.store.UpdateStatus(activeSrv.ID, "online", ping)
				}
				na.app.QueueUpdateDraw(func() {
					na.updateHeader()
				})
			}
		}
	}()
}

// ================= MODAL HELPER =================
func (na *NativeApp) showModal(p tview.Primitive, width, height int) {
	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(p, width, 1, true).
			AddItem(nil, 0, 1, false), height, 1, true).
		AddItem(nil, 0, 1, false)

	na.pages.AddPage("modal", modal, true, true)
	na.app.SetFocus(p)
}

func (na *NativeApp) showInfo(title, msg string) {
	modal := tview.NewModal().
		SetText(msg).
		AddButtons([]string{"Закрыть"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			na.pages.RemovePage("modal")
		})
	modal.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", title))
	na.pages.AddPage("modal", modal, true, true)
	na.app.SetFocus(modal)
}

// ================= SERVER MODAL =================
func (na *NativeApp) showServerModal() {
	list := tview.NewList().ShowSecondaryText(true)
	servers := na.store.List()

	for _, srv := range servers {
		s := srv
		mark := "  "
		if s.IsActive {
			mark = "✓ "
		}
		status := "[green]Online[-]"
		if s.Status == "offline" {
			status = "[red]Offline[-]"
		}
		title := fmt.Sprintf("%s%s (%s)", mark, s.Name, status)
		sub := fmt.Sprintf("   URL: %s | Ping: %dms", s.URL, s.PingMs)

		list.AddItem(title, sub, 0, func() {
			_ = na.store.SetActive(s.ID)
			na.pages.RemovePage("modal")
			na.updateHeader()
			na.refreshDashboard()
		})
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(tview.NewTextView().SetDynamicColors(true).SetText(" [yellow]Enter[-] Выбрать  •  [green]A[-] Добавить узел  •  [red]D[-] Удалить  •  [gray]Esc[-] Закрыть"), 1, 1, false)

	flex.SetBorder(true).SetTitle(" 🖥️ Управление серверами (Узлы Node) ")

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'a', 'A', 'ф', 'Ф':
				na.showAddServerModal()
				return nil
			case 'd', 'D', 'в', 'В':
				idx := list.GetCurrentItem()
				if idx >= 0 && idx < len(servers) {
					delSrv := servers[idx]
					_ = na.store.DeleteServer(delSrv.ID)
					na.showServerModal()
				}
				return nil
			}
		}
		return event
	})

	na.showModal(flex, 70, 18)
}

func (na *NativeApp) showAddServerModal() {
	form := tview.NewForm()
	nameInput := ""
	urlInput := "http://127.0.0.1:9090"
	secretInput := ""
	pubHostInput := ""
	sharePortInput := "9091"

	form.AddInputField("Название:", "VPS 2 (Основной)", 30, nil, func(text string) { nameInput = text })
	form.AddInputField("URL агента:", urlInput, 35, nil, func(text string) { urlInput = text })
	form.AddPasswordField("Secret Key (Auth):", "", 25, '*', func(text string) { secretInput = text })
	form.AddInputField("Публичный IP/Хост (для ссылок):", "", 25, nil, func(text string) { pubHostInput = text })
	form.AddInputField("Порт шаринга файлов:", sharePortInput, 10, nil, func(text string) { sharePortInput = text })

	form.AddButton("Добавить", func() {
		if urlInput == "" {
			na.showInfo("Ошибка", "URL агента обязателен!")
			return
		}
		if nameInput == "" {
			nameInput = "VPS Node"
		}
		_, err := na.store.AddServer(nameInput, urlInput, secretInput, pubHostInput, sharePortInput)
		if err != nil {
			na.showInfo("Ошибка", err.Error())
			return
		}
		na.pages.RemovePage("modal")
		na.updateHeader()
		na.showServerModal()
	})
	form.AddButton("Отмена", func() {
		na.showServerModal()
	})

	form.SetBorder(true).SetTitle(" ➕ Добавить сервер Node ")
	na.showModal(form, 60, 16)
}

// ================= THEME MODAL =================
func (na *NativeApp) showThemeModal() {
	list := tview.NewList()
	themes := []ThemeName{ThemeDark, ThemeLight, ThemeGreen, ThemePurple}

	for _, th := range themes {
		tName := th
		pal := Palettes[tName]
		mark := "   "
		if na.activeTheme == tName {
			mark = " ✓ "
		}
		list.AddItem(mark+pal.Title, "", 0, func() {
			na.activeTheme = tName
			ApplyTheme(tName)
			na.pages.RemovePage("modal")
			na.updateHeader()
		})
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(tview.NewTextView().SetDynamicColors(true).SetText(" [yellow]Enter[-] Применить  •  [gray]Esc[-] Закрыть"), 1, 1, false)

	flex.SetBorder(true).SetTitle(" 🎨 Выбор темы оформления ")
	na.showModal(flex, 45, 10)
}

// ================= VIEW 1: DASHBOARD =================
var dashboardHealthView *tview.TextView
var dashboardActivityView *tview.TextView

func (na *NativeApp) buildDashboardView() {
	dashboardHealthView = tview.NewTextView().SetDynamicColors(true)
	dashboardHealthView.SetBorder(true).SetTitle(" 📊 Состояние активного узла ")

	dashboardActivityView = tview.NewTextView().SetDynamicColors(true)
	dashboardActivityView.SetBorder(true).SetTitle(" ⚡ Быстрый запуск и управление ")

	dashboardActivityView.SetText(`
 [aqua:b]Добро пожаловать в нативный клиент Node Control![-]

 Доступные разделы управления:
  • [yellow][2/F2] Файловый менеджер[-]
    Проводник по серверу, загрузка и скачивание файлов,
    генерация публичных ссылок для шаринга на отдельном порту.

  • [green][3/F3] Базы данных[-]
    Управление СУБД PostgreSQL, MySQL, встроенным KV хранилищем,
    просмотр таблиц и исполнение SQL-запросов в реальном времени.

  • [purple][4/F4] Скрипты и процессы[-]
    Запуск Python, Node.js, Golang скриптов, автоустановка зависимостей,
    прямой стриминг логов и история изменений кода.

 [gray]Управление:[blue] Нажимайте клавиши 1-4 или кликайте мышью.`)

	na.dashboardFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(dashboardHealthView, 11, 1, false).
		AddItem(dashboardActivityView, 0, 1, true)
}

func (na *NativeApp) refreshDashboard() {
	go func() {
		info, ping, err := na.api.GetHealth()
		srv, _ := na.store.GetActive()

		na.app.QueueUpdateDraw(func() {
			if err != nil {
				dashboardHealthView.SetText(fmt.Sprintf("\n [red:b] ❌ СЕРВЕР НЕДОСТУПЕН[-] \n\n Ошибка подключения: %v\n URL узла: %s\n Проверьте работу агента (node-agent) или сеть Tailscale.", err, srv.URL))
				return
			}
			dashboardHealthView.SetText(fmt.Sprintf(`
 [green:b] ✓ Сервер в сети[-] | Узел: [bold]%s[-] | Ping: [aqua]%d ms[-]
 • [bold]Хост:[-]        %s (%s/%s)
 • [bold]Uptime:[-]      %s (Старт: %s)
 • [bold]Версия Go:[-]   %s | Версия Агента: %s
 • [bold]Адрес:[-]       %s | Tailscale защита: активна`,
				srv.Name, ping,
				info.Hostname, info.OS, info.Arch,
				info.Uptime, info.StartTime.Format("02.01.2006 15:04:05"),
				info.GoVersion, info.Version,
				srv.URL,
			))
		})
	}()
}

// ================= VIEW 2: FILES =================
func (na *NativeApp) buildFilesView() {
	na.filesPathLbl = tview.NewInputField().
		SetLabel(" 📁 Путь: ").
		SetFieldWidth(50)

	na.filesTable = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	na.filesPathLbl.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			na.currentPath = na.filesPathLbl.GetText()
			na.refreshFiles()
		}
	})

	actionBar := tview.NewTextView().SetDynamicColors(true).
		SetText(" [yellow]Enter[-] Открыть  •  [green]U[-] Загрузить файл  •  [aqua]D[-] Скачать  •  [purple]S[-] Ссылка  •  [red]X[-] Удалить  •  [yellow]M[-] Папка  •  [white]R[-] Обновить")

	na.filesTable.SetSelectedFunc(func(row, column int) {
		if row == 0 {
			return
		}
		cell := na.filesTable.GetCell(row, 0)
		if cell == nil {
			return
		}
		itemPath := cell.GetReference().(string)
		isDir := strings.HasPrefix(cell.Text, "📁") || cell.Text == "📁 .."
		if isDir {
			na.currentPath = itemPath
			na.filesPathLbl.SetText(itemPath)
			na.refreshFiles()
		} else {
			na.showFileViewModal(itemPath)
		}
	})

	na.filesTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'u', 'U', 'г', 'Г':
				na.showFileUploadModal()
				return nil
			case 'd', 'D', 'в', 'В':
				na.downloadSelectedFile()
				return nil
			case 's', 'S', 'ы', 'Ы':
				na.shareSelectedFile()
				return nil
			case 'x', 'X', 'ч', 'Ч':
				na.deleteSelectedFile()
				return nil
			case 'm', 'M', 'ь', 'Ь':
				na.showMkdirModal()
				return nil
			case 'r', 'R', 'к', 'К':
				na.refreshFiles()
				return nil
			}
		}
		return event
	})

	na.filesFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(na.filesPathLbl, 1, 1, false).
		AddItem(na.filesTable, 0, 1, true).
		AddItem(actionBar, 1, 1, false)

	na.filesFlex.SetBorder(true).SetTitle(" 📂 Файловый менеджер сервера ")
}

func (na *NativeApp) refreshFiles() {
	go func() {
		listing, err := na.api.ListFiles(na.currentPath)
		na.app.QueueUpdateDraw(func() {
			na.filesTable.Clear()
			if err != nil {
				na.filesTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[red]Ошибка: %v", err)))
				return
			}

			na.currentPath = listing.CurrentPath
			na.filesPathLbl.SetText(listing.CurrentPath)

			// Table Header
			na.filesTable.SetCell(0, 0, tview.NewTableCell("[yellow:b]Имя файла/папки[-]").SetSelectable(false))
			na.filesTable.SetCell(0, 1, tview.NewTableCell("[yellow:b]Тип[-]").SetSelectable(false))
			na.filesTable.SetCell(0, 2, tview.NewTableCell("[yellow:b]Размер[-]").SetSelectable(false))
			na.filesTable.SetCell(0, 3, tview.NewTableCell("[yellow:b]Изменён[-]").SetSelectable(false))

			row := 1
			// Parent dir
			parent := filepath.Dir(na.currentPath)
			if parent != na.currentPath {
				c := tview.NewTableCell("📁 ..").SetReference(parent).SetTextColor(tcell.ColorYellow)
				na.filesTable.SetCell(row, 0, c)
				na.filesTable.SetCell(row, 1, tview.NewTableCell("Папка"))
				na.filesTable.SetCell(row, 2, tview.NewTableCell("-"))
				na.filesTable.SetCell(row, 3, tview.NewTableCell("-"))
				row++
			}

			for _, it := range listing.Items {
				name := it.Name
				typeStr := "Файл"
				sizeStr := formatFileSize(it.Size)
				color := tcell.ColorWhite
				if it.IsDir {
					name = "📁 " + name
					typeStr = "Папка"
					sizeStr = "-"
					color = tcell.ColorAqua
				} else {
					name = "📄 " + name
				}

				c := tview.NewTableCell(name).SetReference(it.Path).SetTextColor(color)
				na.filesTable.SetCell(row, 0, c)
				na.filesTable.SetCell(row, 1, tview.NewTableCell(typeStr))
				na.filesTable.SetCell(row, 2, tview.NewTableCell(sizeStr))
				na.filesTable.SetCell(row, 3, tview.NewTableCell(it.ModTime.Format("02.01 15:04")))
				row++
			}
		})
	}()
}

func (na *NativeApp) showFileUploadModal() {
	form := tview.NewForm()
	localPath := ""
	form.AddInputField("Путь к файлу на этом компьютере:", "", 50, nil, func(t string) { localPath = t })
	form.AddButton("Загрузить на сервер", func() {
		if localPath == "" {
			na.showInfo("Ошибка", "Укажите локальный путь к файлу!")
			return
		}
		na.pages.RemovePage("modal")
		go func() {
			err := na.api.UploadFile(localPath, na.currentPath)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка загрузки", err.Error())
				} else {
					na.showInfo("Успех", "Файл успешно загружен на сервер!")
					na.refreshFiles()
				}
			})
		}()
	})
	form.AddButton("Отмена", func() { na.pages.RemovePage("modal") })
	form.SetBorder(true).SetTitle(" ⬆️ Загрузка файла на сервер ")
	na.showModal(form, 65, 9)
}

func (na *NativeApp) downloadSelectedFile() {
	row, _ := na.filesTable.GetSelection()
	if row <= 0 {
		return
	}
	cell := na.filesTable.GetCell(row, 0)
	if cell == nil || strings.HasPrefix(cell.Text, "📁") {
		return
	}
	remotePath := cell.GetReference().(string)
	baseName := filepath.Base(remotePath)

	form := tview.NewForm()
	savePath := baseName
	form.AddInputField("Сохранить как (локальный путь):", savePath, 45, nil, func(t string) { savePath = t })
	form.AddButton("Скачать", func() {
		na.pages.RemovePage("modal")
		go func() {
			err := na.api.DownloadFile(remotePath, savePath)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка скачивания", err.Error())
				} else {
					na.showInfo("Скачано", fmt.Sprintf("Файл сохранён в: %s", savePath))
				}
			})
		}()
	})
	form.AddButton("Отмена", func() { na.pages.RemovePage("modal") })
	form.SetBorder(true).SetTitle(" ⬇️ Скачивание файла ")
	na.showModal(form, 60, 9)
}

func (na *NativeApp) shareSelectedFile() {
	row, _ := na.filesTable.GetSelection()
	if row <= 0 {
		return
	}
	cell := na.filesTable.GetCell(row, 0)
	if cell == nil || strings.HasPrefix(cell.Text, "📁") {
		return
	}
	remotePath := cell.GetReference().(string)

	go func() {
		link, err := na.api.ShareFile(remotePath)
		na.app.QueueUpdateDraw(func() {
			if err != nil {
				na.showInfo("Ошибка", err.Error())
				return
			}
			msg := fmt.Sprintf("Публичная ссылка на файл (без Tailscale):\n\n[yellow]%s[-]\n\nСсылка доступна с любого устройства в интернете!", link)
			na.showInfo("🔗 Ссылка для шаринга", msg)
		})
	}()
}

func (na *NativeApp) deleteSelectedFile() {
	row, _ := na.filesTable.GetSelection()
	if row <= 0 {
		return
	}
	cell := na.filesTable.GetCell(row, 0)
	if cell == nil || cell.Text == "📁 .." {
		return
	}
	targetPath := cell.GetReference().(string)

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Удалить файл/папку:\n\n%s?", filepath.Base(targetPath))).
		AddButtons([]string{"Удалить", "Отмена"}).
		SetDoneFunc(func(btnIndex int, btnLabel string) {
			na.pages.RemovePage("modal")
			if btnLabel == "Удалить" {
				go func() {
					err := na.api.DeleteFile(targetPath)
					na.app.QueueUpdateDraw(func() {
						if err != nil {
							na.showInfo("Ошибка удаления", err.Error())
						} else {
							na.refreshFiles()
						}
					})
				}()
			}
		})
	modal.SetBorder(true).SetTitle(" 🗑️ Подтверждение удаления ")
	na.pages.AddPage("modal", modal, true, true)
	na.app.SetFocus(modal)
}

func (na *NativeApp) showMkdirModal() {
	form := tview.NewForm()
	dirName := ""
	form.AddInputField("Имя новой папки:", "", 35, nil, func(t string) { dirName = t })
	form.AddButton("Создать", func() {
		if dirName == "" {
			return
		}
		na.pages.RemovePage("modal")
		go func() {
			target := filepath.Join(na.currentPath, dirName)
			err := na.api.Mkdir(target)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка", err.Error())
				} else {
					na.refreshFiles()
				}
			})
		}()
	})
	form.AddButton("Отмена", func() { na.pages.RemovePage("modal") })
	form.SetBorder(true).SetTitle(" 📁 Создание папки ")
	na.showModal(form, 50, 9)
}

func (na *NativeApp) showFileViewModal(path string) {
	txtView := tview.NewTextView().SetDynamicColors(false)
	txtView.SetBorder(true).SetTitle(fmt.Sprintf(" Просмотр: %s ", filepath.Base(path)))

	tmpSave := filepath.Join(".", ".tmp_preview")
	go func() {
		err := na.api.DownloadFile(path, tmpSave)
		if err == nil {
			bytes, _ := os.ReadFile(tmpSave)
			_ = os.Remove(tmpSave)
			na.app.QueueUpdateDraw(func() {
				txtView.SetText(string(bytes))
			})
		}
	}()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(txtView, 0, 1, true).
		AddItem(tview.NewButton("Закрыть").SetSelectedFunc(func() { na.pages.RemovePage("modal") }), 1, 1, false)

	na.showModal(flex, 80, 24)
}

func formatFileSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ================= VIEW 3: DATABASES =================
func (na *NativeApp) buildDBView() {
	na.dbTable = tview.NewTable().SetBorders(true).SetSelectable(true, false)
	na.dbTable.SetBorder(true).SetTitle(" 🗄️ Базы данных (PostgreSQL / MySQL / KV) ")

	na.dbQueryArea = tview.NewTextArea()
	na.dbQueryArea.SetPlaceholder("Введите SQL запрос (например: SELECT * FROM users LIMIT 10;)")
	na.dbQueryArea.SetBorder(true).SetTitle(" SQL Запрос ")

	na.dbResTable = tview.NewTable().SetBorders(true).SetSelectable(true, false)
	na.dbResTable.SetBorder(true).SetTitle(" Результаты запроса ")

	runSqlBtn := tview.NewButton("▶ Выполнить SQL").SetSelectedFunc(func() {
		na.runSQLQuery()
	})

	addDbBtn := tview.NewButton("➕ Создать базу").SetSelectedFunc(func() {
		na.showAddDBModal()
	})

	dropDbBtn := tview.NewButton("🗑️ Удалить базу").SetSelectedFunc(func() {
		na.dropSelectedDB()
	})

	btnRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(runSqlBtn, 18, 1, false).
		AddItem(addDbBtn, 18, 1, false).
		AddItem(dropDbBtn, 18, 1, false)

	// KV Panel
	na.kvKeyInput = tview.NewInputField().SetLabel("KV Путь/Ключ: ").SetFieldWidth(25)
	na.kvValInput = tview.NewTextArea()
	na.kvValInput.SetBorder(true).SetTitle(" KV Значение ")

	kvReadBtn := tview.NewButton("📥 Читать").SetSelectedFunc(func() { na.kvRead() })
	kvWriteBtn := tview.NewButton("💾 Записать").SetSelectedFunc(func() { na.kvWrite() })

	kvBtnRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(na.kvKeyInput, 35, 1, false).
		AddItem(kvReadBtn, 12, 1, false).
		AddItem(kvWriteBtn, 14, 1, false)

	leftPane := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(na.dbTable, 0, 1, true).
		AddItem(btnRow, 1, 1, false)

	rightPane := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(na.dbQueryArea, 6, 1, false).
		AddItem(na.dbResTable, 0, 1, false).
		AddItem(kvBtnRow, 1, 1, false).
		AddItem(na.kvValInput, 6, 1, false)

	na.dbFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftPane, 35, 1, true).
		AddItem(rightPane, 0, 2, false)

	na.dbFlex.SetBorder(true).SetTitle(" 🗄️ Базы данных и KV хранилище ")
}

func (na *NativeApp) refreshDB() {
	go func() {
		list, err := na.api.ListDatabases()
		na.app.QueueUpdateDraw(func() {
			na.dbTable.Clear()
			if err != nil {
				na.dbTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[red]Ошибка: %v", err)))
				return
			}
			na.dbTable.SetCell(0, 0, tview.NewTableCell("[yellow:b]Имя базы[-]").SetSelectable(false))
			na.dbTable.SetCell(0, 1, tview.NewTableCell("[yellow:b]Тип[-]").SetSelectable(false))

			// Always add built-in KV
			na.dbTable.SetCell(1, 0, tview.NewTableCell("KV Built-in").SetReference("kv"))
			na.dbTable.SetCell(1, 1, tview.NewTableCell("[aqua]KV Storage[-]"))

			row := 2
			for _, db := range list {
				c := tview.NewTableCell(db.Name).SetReference(db.ID)
				tColor := "[green]"
				if db.Type == "mysql" {
					tColor = "[blue]"
				}
				na.dbTable.SetCell(row, 0, c)
				na.dbTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%s%s[-]", tColor, db.Type)))
				row++
			}
		})
	}()
}

func (na *NativeApp) showAddDBModal() {
	form := tview.NewForm()
	dbType := "postgres"
	name := "my_database"
	conn := "postgres://user:password@localhost:5432/dbname?sslmode=disable"

	form.AddDropDown("Тип СУБД:", []string{"PostgreSQL", "MySQL", "KV"}, 0, func(option string, index int) {
		switch index {
		case 0:
			dbType = "postgres"
		case 1:
			dbType = "mysql"
		case 2:
			dbType = "kv"
		}
	})
	form.AddInputField("Имя БД:", name, 25, nil, func(t string) { name = t })
	form.AddInputField("Строка подключения:", conn, 45, nil, func(t string) { conn = t })

	form.AddButton("Создать", func() {
		na.pages.RemovePage("modal")
		go func() {
			err := na.api.CreateDatabase(dbType, name, conn)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка создания", err.Error())
				} else {
					na.refreshDB()
				}
			})
		}()
	})
	form.AddButton("Отмена", func() { na.pages.RemovePage("modal") })
	form.SetBorder(true).SetTitle(" ➕ Создать/подключить базу данных ")
	na.showModal(form, 65, 12)
}

func (na *NativeApp) dropSelectedDB() {
	row, _ := na.dbTable.GetSelection()
	if row <= 1 {
		na.showInfo("Внимание", "Нельзя удалить встроенную KV базу!")
		return
	}
	cell := na.dbTable.GetCell(row, 0)
	if cell == nil {
		return
	}
	dbId := cell.GetReference().(string)

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Удалить базу данных %s?", cell.Text)).
		AddButtons([]string{"Удалить", "Отмена"}).
		SetDoneFunc(func(btnIndex int, btnLabel string) {
			na.pages.RemovePage("modal")
			if btnLabel == "Удалить" {
				go func() {
					err := na.api.DropDatabase(dbId)
					na.app.QueueUpdateDraw(func() {
						if err != nil {
							na.showInfo("Ошибка", err.Error())
						} else {
							na.refreshDB()
						}
					})
				}()
			}
		})
	na.pages.AddPage("modal", modal, true, true)
	na.app.SetFocus(modal)
}

func (na *NativeApp) runSQLQuery() {
	row, _ := na.dbTable.GetSelection()
	dbId := "default"
	if row > 0 {
		cell := na.dbTable.GetCell(row, 0)
		if cell != nil && cell.GetReference() != nil {
			dbId = cell.GetReference().(string)
		}
	}
	query := na.dbQueryArea.GetText()
	if query == "" {
		na.showInfo("Запрос", "Введите SQL запрос для выполнения!")
		return
	}

	go func() {
		res, err := na.api.QueryDatabase(dbId, query)
		na.app.QueueUpdateDraw(func() {
			na.dbResTable.Clear()
			if err != nil {
				na.dbResTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[red]Ошибка SQL: %v", err)))
				return
			}
			if res.Error != "" {
				na.dbResTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[red]%s", res.Error)))
				return
			}

			// Render Columns
			for cIdx, col := range res.Columns {
				na.dbResTable.SetCell(0, cIdx, tview.NewTableCell(fmt.Sprintf("[yellow:b]%s[-]", col)).SetSelectable(false))
			}

			// Render Rows
			for rIdx, rData := range res.Rows {
				for cIdx, val := range rData {
					na.dbResTable.SetCell(rIdx+1, cIdx, tview.NewTableCell(fmt.Sprintf("%v", val)))
				}
			}
		})
	}()
}

func (na *NativeApp) kvRead() {
	path := na.kvKeyInput.GetText()
	go func() {
		val, err := na.api.KVRead("default", path)
		na.app.QueueUpdateDraw(func() {
			if err != nil {
				na.showInfo("Ошибка KV", err.Error())
			} else {
				na.kvValInput.SetText(val, true)
			}
		})
	}()
}

func (na *NativeApp) kvWrite() {
	path := na.kvKeyInput.GetText()
	val := na.kvValInput.GetText()
	go func() {
		err := na.api.KVWrite("default", path, val, false, false)
		na.app.QueueUpdateDraw(func() {
			if err != nil {
				na.showInfo("Ошибка KV", err.Error())
			} else {
				na.showInfo("KV", "Значение успешно сохранено!")
			}
		})
	}()
}

// ================= VIEW 4: SCRIPTS =================
func (na *NativeApp) buildScriptsView() {
	na.scriptIdInput = tview.NewInputField().
		SetLabel(" ID скрипта: ").
		SetText("main").
		SetFieldWidth(20)

	na.scriptLangDrop = tview.NewDropDown().
		SetLabel(" Язык: ").
		SetOptions([]string{"Python (.py)", "Node.js (.js)", "Golang (.go)"}, nil).
		SetCurrentOption(0)

	na.scriptEditor = tview.NewTextArea()
	na.scriptEditor.SetBorder(true).SetTitle(" 📝 Редактор скрипта ")
	na.scriptEditor.SetText(`# Пример Python скрипта
import time
print("Привет от Node Agent!")
for i in range(5):
    print(f"Шаг {i + 1}...")
    time.Sleep(1)
print("Готово!")
`, false)

	na.scriptLogsView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	na.scriptLogsView.SetBorder(true).SetTitle(" 📜 Логи выполнения (Real-Time) ")

	na.scriptRunBtn = tview.NewButton("▶ Запустить скрипт").SetSelectedFunc(func() {
		na.toggleScriptRun()
	})

	saveBtn := tview.NewButton("💾 Сохранить").SetSelectedFunc(func() {
		na.saveCurrentScript()
	})

	topControlBar := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(na.scriptIdInput, 25, 1, false).
		AddItem(na.scriptLangDrop, 25, 1, false).
		AddItem(na.scriptRunBtn, 24, 1, false).
		AddItem(saveBtn, 16, 1, false)

	autoDepNotice := tview.NewTextView().SetDynamicColors(true).
		SetText(" [aqua]ℹ Автоустановка зависимостей активна (pip / npm / go get запускаются автоматически при необходимости)[-]")

	leftPane := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topControlBar, 1, 1, false).
		AddItem(autoDepNotice, 1, 1, false).
		AddItem(na.scriptEditor, 0, 1, true)

	na.scriptsFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(leftPane, 0, 1, true).
		AddItem(na.scriptLogsView, 12, 1, false)

	na.scriptsFlex.SetBorder(true).SetTitle(" ⚙️ Запуск скриптов с автозависимостями ")
}

func (na *NativeApp) refreshScripts() {
	id := na.scriptIdInput.GetText()
	if id == "" {
		id = "main"
	}
	go func() {
		s, err := na.api.GetScript(id)
		if err == nil && s != nil {
			na.app.QueueUpdateDraw(func() {
				if s.Code != "" {
					na.scriptEditor.SetText(s.Code, false)
				}
				na.isScriptRun = s.IsRunning
				na.updateScriptRunButton()
			})
		}
	}()
	na.startLogsStreaming(id)
}

func (na *NativeApp) getSelectedLang() string {
	idx, _ := na.scriptLangDrop.GetCurrentOption()
	switch idx {
	case 1:
		return "node"
	case 2:
		return "go"
	default:
		return "python"
	}
}

func (na *NativeApp) saveCurrentScript() {
	id := na.scriptIdInput.GetText()
	lang := na.getSelectedLang()
	code := na.scriptEditor.GetText()
	go func() {
		err := na.api.SaveScript(id, lang, code)
		na.app.QueueUpdateDraw(func() {
			if err != nil {
				na.showInfo("Ошибка сохранения", err.Error())
			} else {
				na.showInfo("Сохранено", fmt.Sprintf("Скрипт '%s' успешно сохранён!", id))
			}
		})
	}()
}

func (na *NativeApp) toggleScriptRun() {
	id := na.scriptIdInput.GetText()
	lang := na.getSelectedLang()
	code := na.scriptEditor.GetText()

	if !na.isScriptRun {
		// Run script
		go func() {
			err := na.api.RunScript(id, lang, code)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка запуска", err.Error())
				} else {
					na.isScriptRun = true
					na.updateScriptRunButton()
				}
			})
		}()
	} else {
		// Stop script
		go func() {
			err := na.api.StopScript(id)
			na.app.QueueUpdateDraw(func() {
				if err != nil {
					na.showInfo("Ошибка остановки", err.Error())
				} else {
					na.isScriptRun = false
					na.updateScriptRunButton()
				}
			})
		}()
	}
}

func (na *NativeApp) updateScriptRunButton() {
	if na.isScriptRun {
		na.scriptRunBtn.SetLabel("■ Остановить скрипт")
	} else {
		na.scriptRunBtn.SetLabel("▶ Запустить скрипт")
	}
}

func (na *NativeApp) startLogsStreaming(id string) {
	go func() {
		for i := 0; i < 30; i++ {
			logs, isRunning, err := na.api.GetScriptLogs(id)
			if err == nil {
				na.app.QueueUpdateDraw(func() {
					na.scriptLogsView.SetText(logs)
					na.scriptLogsView.ScrollToEnd()
					na.isScriptRun = isRunning
					na.updateScriptRunButton()
				})
			}
			time.Sleep(1 * time.Second)
		}
	}()
}
