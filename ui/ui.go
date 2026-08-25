package ui

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"github.com/termoose/irccloud/config"
	"github.com/termoose/irccloud/requests"
	"log"
	"strings"
	"sync"
)

type View struct {
	basePages   *tview.Pages
	pages       *tview.Pages
	layout      *tview.Grid
	app         *tview.Application
	channels    channelList
	websocket   *requests.Connection
	config      *config.Data
	Activity    *activityBar
	lastChan    string
	channelLock sync.Mutex
	pickerKey   tcell.Key
}

func floatingModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(nil, 0, 1, false), width, 1, false).
		AddItem(nil, 0, 1, false)
}

func NewView(socket *requests.Connection, c *config.Data) *View {
	view := &View{
		pages:     tview.NewPages(),
		layout:    newGrid(),
		basePages: tview.NewPages(),
		websocket: socket,
		config:    c,
		Activity:  NewActivityBar(c.Triggers),
		lastChan:  c.LastChan,
		pickerKey: resolvePickerKey(c.ChannelPickerKey),
	}

	return view
}

// resolvePickerKey falls back to the default when the config is silent or
// names a key that cannot be used, reporting why rather than leaving the
// user with a binding that silently never fires.
func resolvePickerKey(configured string) tcell.Key {
	if strings.TrimSpace(configured) == "" {
		key, _ := ParseKey(DefaultChannelPickerKey)
		return key
	}

	key, err := ParseKey(configured)
	if err != nil {
		log.Printf("channel_picker_key: %v; falling back to %s", err, DefaultChannelPickerKey)

		key, _ = ParseKey(DefaultChannelPickerKey)
		return key
	}

	return key
}

func (v *View) GetCurrentChannel() string {
	name, _ := v.pages.GetFrontPage()
	return name
}

func (v *View) Start() {
	v.app = tview.NewApplication()

	v.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})

	v.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == v.pickerKey || event.Key() == tcell.KeyCtrlSpace {
			if v.basePages.HasPage("select_channel") {
				v.hideChannelSelector()
			} else {
				v.showChannelSelector()
			}
		}

		if event.Key() == tcell.KeyCtrlB {
			lastActive, err := v.Activity.GetLatestActiveChannel()

			if err == nil {
				_, channel := v.getChannelByName(lastActive)

				if channel != nil {
					v.gotoPage(channel)
				}
			}
		}

		if event.Key() == tcell.KeyPgUp {
			channelName := v.GetCurrentChannel()
			_, channel := v.getChannelByName(channelName)

			channel.Scroll(-10)
		}

		if event.Key() == tcell.KeyPgDn {
			channelName := v.GetCurrentChannel()
			_, channel := v.getChannelByName(channelName)

			channel.Scroll(10)
		}

		return event
	})

	v.basePages.AddPage("channel", v.pages, true, true)
	// 	true, true)

	v.layout.AddItem(v.basePages, 1, 0, 1, 1, 0, 0, true)
	v.layout.AddItem(v.Activity.bar, 0, 0, 1, 1, 0, 0, false)

	if err := v.app.
		SetRoot(v.layout, true).
		SetFocus(v.layout).
		Run(); err != nil {
		panic(err)
	}
}

func (v *View) HideSplash() {
	v.basePages.RemovePage("splash")
}

func (v *View) Stop() {
	v.app.Stop()
}

func (v *View) SetLatestChannel() {
	_, selected := v.getChannelByName(v.lastChan)

	if selected != nil {
		v.app.QueueUpdate(func() {
			v.Activity.MarkAsVisited(selected.name, v)
			v.pages.SwitchToPage(selected.name)
			v.app.SetFocus(selected.input)
		})
	}
}

func (v *View) Redraw() {
	v.app.Draw()
}

func (v *View) sendToBuffer(cid int, channel, message string) {
	v.websocket.SendMessage(cid, channel, message)
}

func newTextInput() *tview.InputField {
	return tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorDimGray).
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholderTextColor(tcell.ColorWhiteSmoke).
		SetPlaceholder("type here...")
}

func newListView() *tview.List {
	return tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true).
		SetMainTextColor(tcell.ColorLightSkyBlue)
}

func newGrid() *tview.Grid {
	return tview.NewGrid().
		SetRows(1, 0).
		SetColumns(0)
}

func newTextView(text string) *tview.TextView {
	return tview.NewTextView().
		SetText(text).
		SetDynamicColors(true).
		SetWordWrap(true)
}
