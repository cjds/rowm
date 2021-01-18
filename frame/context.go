package frame

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/wingo/misc"
	"github.com/BurntSushi/wingo/prompt"
	"github.com/BurntSushi/xgb/xinerama"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xcursor"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/google/goexpect"
	"github.com/levavakian/rowm/ext"
	"github.com/levavakian/rowm/sideloop"
	"log"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

// NoFont is a font where all the characters are a circle.
// This is because I am lazy and didn't want to modify the wingo prompt
// for password related stuff, so instead we show passwords with a uniform font.
var NoFont = xgraphics.MustFont(xgraphics.ParseFont(
	bytes.NewBuffer(misc.DataFile("write-your-password-with-this-font.ttf"))))

type AttachTarget struct {
	Target *Frame
	Type   PartitionType
}

type Yank struct {
	Container *Container
	Window    xproto.Window
}

// Context represents all non-trivial state stored by the window manager.
type Context struct {
	X                      *xgbutil.XUtil                    // The connection to X
	AttachPoint            *AttachTarget                     // The frame to split when adding the next window (if any)
	Yanked                 *Yank                             // The window or container selected to transfer (if any)
	Tracked                map[xproto.Window]*Frame          // All known user windows
	UnmapCounter           map[xproto.Window]int             // Tracking of unmap notifications to distinguish internal from external
	Containers             map[*Container]struct{}           // All known containers
	Cursors                map[int]xproto.Cursor             // X cursor cache
	DummyIcon              *xgraphics.Image                  // Icon to use for no icon
	Backgrounds            map[xproto.Window]*xwindow.Window // Background windows
	Config                 Config                            // All user provided preferences
	Screens                []Rect                            // All heads (aka monitors) and their shapes
	LastKnownFocused       xproto.Window                     // Last window we knew of that had input focus
	LastKnownFocusedScreen int                               // Last screen/head/monitor that had a focused window that we know of
	SplitPrompt            *InputWithSuggestion              // Prompt for splitting windows (if any active)
	Locked                 bool                              // Whether we should be in a lock screen
	LockPrompt             *prompt.Input                     // Prompt for unlocking screen (if any)
	Taskbar                *Taskbar                          // The taskbar, doesn't need a comment but it felt lonely
	FocusMarker            *xwindow.Window                   // Marker for recently focused windows when cycling
	LastLockChange         time.Time                         // Last time we went from locked->unlocked or reverse
	Injector               *sideloop.Injector                // Utility for inserting work between X events
	Gotos                  map[string]xproto.Window          // Mapping of shortcut minimize/focus keys for windows
	RightClickMenu         *RightClickMenu			 // Right Click Menu, is a menu, when you right click
	AlwaysOnTop            map[xproto.Window]*Container      // List of Always On Top Windows
}

// NewContext will create a new context but also populate screen backgrounds, create the taskbar, and generate the cursor cache
func NewContext(x *xgbutil.XUtil, inj *sideloop.Injector) (*Context, error) {
	conf := LoadConfig()

	var err error
	c := &Context{
		X:              x,
		Tracked:        make(map[xproto.Window]*Frame),
		UnmapCounter:   make(map[xproto.Window]int),
		Cursors:        make(map[int]xproto.Cursor),
		Containers:     make(map[*Container]struct{}),
		Config:         conf,
		DummyIcon:      xgraphics.New(x, conf.TaskbarElementShape.ToImageRect()),
		LastLockChange: time.Now(),
		Injector:       inj,
		Gotos:          make(map[string]xproto.Window),
	}
	c.UpdateScreens()
	c.Taskbar = NewTaskbar(c)
	c.AlwaysOnTop = make(map[xproto.Window]*Container)
	if err != nil {
		log.Fatal(err)
	}
	for i := xcursor.XCursor; i < xcursor.XTerm; i++ {
		curs, err := xcursor.CreateCursor(x, uint16(i))
		if err != nil {
			break
		}
		c.Cursors[i] = curs
	}
	return c, err
}

func (ctx *Context) GenerateLockPrompt() {
	theme := *prompt.DefaultInputTheme
	theme.Font = NoFont
	lockPrompt := prompt.NewInput(ctx.X, &theme, prompt.DefaultInputConfig)
	ctx.LockPrompt = lockPrompt

	canc := func(inp *prompt.Input) {
		// Only regenerate the lock if it's us, so we don't get into infinite loops
		if ctx.LockPrompt != lockPrompt {
			return
		}
		ctx.RaiseLock()
	}

	resp := func(inp *prompt.Input, text string) {
		usr, err := user.Current()
		if err != nil {
			log.Println(err)
			return
		}

		e, _, err := expect.Spawn(fmt.Sprintf("su - %s", usr.Name), time.Second*10)
		if err != nil {
			log.Println(err)
			return
		}
		defer e.Close()

		// Try to log in with the user provided password, if it succeeds, lift the lock
		outs, err := e.ExpectBatch([]expect.Batcher{
			&expect.BExp{R: "Password:"},
			&expect.BSnd{S: text + "\n"},
			&expect.BExp{R: ".+"},
		}, time.Second*10)

		ok := true
		if err != nil {
			ok = false
			log.Println(err)
		} else if len(outs) != 2 {
			log.Println("wrong amount of outputs")
			ok = false
		} else if !strings.Contains(outs[1].Output, usr.Name) {
			log.Println("user name not in login outs")
			ok = false
		}

		if ok {
			ctx.SetLocked(false)
		}

		ctx.LockPrompt.Destroy()
		ctx.LockPrompt = nil
		if ctx.Locked {
			ctx.RaiseLock()
		} else {
			ctx.LowerLock()
		}
	}
	ctx.LockPrompt.Show(ctx.Screens[0].ToXRect(), "", resp, canc)
}

func (ctx *Context) RaiseLock() {
	if !ctx.Locked {
		return
	}

	for _, bg := range ctx.Backgrounds {
		bg.Stack(xproto.StackModeAbove)
	}
	ext.Focus(xwindow.New(ctx.X, ctx.X.Dummy()))

	ctx.GenerateLockPrompt()
}

func (ctx *Context) LowerLock() {
	if ctx.Locked {
		return
	}

	for _, bg := range ctx.Backgrounds {
		bg.Stack(xproto.StackModeBelow)
	}
}

func (ctx *Context) SetLocked(state bool) {
	if state == ctx.Locked {
		return
	}
	ctx.Locked = state
	if ctx.Locked {
		ctx.RaiseLock()
		err := exec.Command("bash", "-c", ctx.Config.SuspendCommand).Run()
		if err != nil {
			log.Println(err)
		}
	} else {
		ctx.LowerLock()
	}
}

func (ctx *Context) DetectScreensChange() (bool, []Rect, error) {
	var Xin []xinerama.ScreenInfo
	if xin, err := xinerama.QueryScreens(ctx.X.Conn()).Reply(); err != nil {
		return false, nil, err
	} else {
		Xin = xin.ScreenInfo
	}

	screens := make([]Rect, 0, len(Xin))
	for _, xi := range Xin {
		screens = append(screens, Rect{
			X: int(xi.XOrg),
			Y: int(xi.YOrg),
			W: int(xi.Width),
			H: int(xi.Height),
		})
	}

	if len(screens) != len(ctx.Screens) {
		return true, screens, nil
	}

	for i := 0; i < len(screens); i++ {
		if screens[i] != ctx.Screens[i] {
			return true, screens, nil
		}
	}
	return false, ctx.Screens, nil
}

func (ctx *Context) UpdateScreens() {
	changed, screens, err := ctx.DetectScreensChange()
	ext.Logerr(err)
	if err != nil || !changed {
		return
	}
	log.Println("found", len(screens), "screen(s)", screens)

	ctx.Screens = screens
	GenerateBackgrounds(ctx)
	for c, _ := range ctx.Containers {
		topShape := TopShape(ctx, c.Shape)
		if screen, overlap, _ := ctx.GetScreenForShape(topShape); topShape.Area() > overlap {
			c.MoveResizeShape(ctx, ctx.DefaultShapeForScreen(screen))
		}
	}

	if ctx.Taskbar != nil {
		ctx.Taskbar.MoveResize(ctx)
	}
	ctx.RaiseLock()
}

func (c *Context) Get(w xproto.Window) *Frame {
	f, _ := c.Tracked[w]
	return f
}

func (ctx *Context) GetScreenForShape(shape Rect) (Rect, int, int) {
	max_overlap := 0
	max_i := 0
	screen := ctx.Screens[0]
	for i, s := range ctx.Screens {
		overlap := AreaOfIntersection(shape, s)
		if overlap > max_overlap {
			max_overlap = overlap
			max_i = i
			screen = s
		}
	}
	return screen, max_overlap, max_i
}

func (ctx *Context) DefaultShapeForScreen(screen Rect) Rect {
	osize := ctx.Config.ElemSize * 2
	offset := 0
	for {
		s := Rect{
			X: screen.X + int(ctx.Config.DefaultShapeRatio.X*float64(screen.W)) + offset*osize,
			Y: screen.Y + int(ctx.Config.DefaultShapeRatio.Y*float64(screen.H)) + offset*osize,
			W: int(ctx.Config.DefaultShapeRatio.W * float64(screen.W)),
			H: int(ctx.Config.DefaultShapeRatio.H * float64(screen.H)),
		}
		tshape := TopShape(ctx, s)
		tscreen, overlap, _ := ctx.GetScreenForShape(tshape)
		if tscreen != screen || overlap < tshape.Area() {
			break
		}

		clear := true
		for c, _ := range ctx.Containers {
			if c.Shape == s {
				clear = false
				break
			}
		}
		if clear {
			return s
		}
		offset++
	}
	return Rect{
		X: screen.X + int(ctx.Config.DefaultShapeRatio.X*float64(screen.W)),
		Y: screen.Y + int(ctx.Config.DefaultShapeRatio.Y*float64(screen.H)),
		W: int(ctx.Config.DefaultShapeRatio.W * float64(screen.W)),
		H: int(ctx.Config.DefaultShapeRatio.H * float64(screen.H)),
	}
}

func (ctx *Context) LastFocusedScreen() Rect {
	if len(ctx.Screens) <= ctx.LastKnownFocusedScreen {
		ctx.LastKnownFocusedScreen = 0
	}
	return ctx.Screens[ctx.LastKnownFocusedScreen]
}

func (ctx *Context) GetFocusedFrame() *Frame {
	// Try getting the input focus directly
	focus, err := xproto.GetInputFocus(ctx.X.Conn()).Reply()
	if err != nil {
		return nil
	}

	found, ok := ctx.Tracked[focus.Focus]
	if ok {
		return found
	}

	// Try the parent as well
	parent, err := xwindow.New(ctx.X, focus.Focus).Parent()
	if err == nil {
		found, ok = ctx.Tracked[parent.Id]
		if ok {
			return found
		}
	}

	// Try our last known focus
	found, ok = ctx.Tracked[ctx.LastKnownFocused]
	if ok {
		return found
	}

	return nil
}
