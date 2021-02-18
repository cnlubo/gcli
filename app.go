package gcli

import (
	"os"
)

// Version the gCli version
var Version = "3.0.1"

/*************************************************************
 * CLI application
 *************************************************************/

// HookFunc definition.
// func arguments:
//  in app, like: func(app *App, data interface{})
//  in cmd, like: func(cmd *Command, data interface{})
// type HookFunc func(obj interface{}, data interface{})
type HookFunc func(obj ...interface{})

// Logo app logo, ASCII logo
type Logo struct {
	Text  string // ASCII logo string
	Style string // eg "info"
}

// App the cli app definition
type App struct {
	// internal use
	core
	// *cmdLine
	// HelpVars
	// Hooks // allow hooks: "init", "before", "after", "error"
	commandBase

	// Name app name
	Name string
	// Desc app description
	Desc string
	// Version app version. like "1.0.1"
	// Version string
	// Logo ASCII logo setting
	Logo Logo
	// Args default is equals to os.args
	Args []string
	// ExitOnEnd call os.Exit on running end
	ExitOnEnd bool
	// ExitFunc default is os.Exit
	ExitFunc func(int)
	// vars you can add some vars map for render help info
	// vars map[string]string
	// command names. key is name, value is name string length
	// eg. {"test": 4, "example": 7}
	names map[string]int
	// store some runtime errors
	errors []error
	// command aliases map. {alias: name}
	aliases map[string]string
	// all commands for the app
	commands map[string]*Command
	// all commands by module
	moduleCommands map[string]map[string]*Command
	// the max length for added command names. default set 12.
	nameMaxLen int
	// default command name
	defaultCommand string
	// raw input command name
	rawName     string
	rawFlagArgs []string
	// clean os.args, not contains bin-name and command-name
	cleanArgs []string
	// current command name
	commandName string
}

// NewApp create new app instance.
// Usage:
// 	NewApp()
// 	// Or with a config func
// 	NewApp(func(a *App) {
// 		// do something before init ....
// 		a.Hooks[gcli.EvtInit] = func () {}
// 	})
func NewApp(fn ...func(app *App)) *App {
	app := &App{
		Args: os.Args,
		Name: "GCli App",
		Desc: "This is my console application",
		Logo: Logo{Style: "info"},
		// set a default version
		// Version: "1.0.0",
		// config
		ExitOnEnd: true,
		// commands
		commands: make(map[string]*Command),
		// group
		moduleCommands: make(map[string]map[string]*Command),
		// some default values
		nameMaxLen: 12,
	}

	// set a default version
	app.Version = "1.0.0"
	// internal core
	app.core = core{
		cmdLine: CLI,
		gFlags: NewFlags("app.GlobalOpts").WithOption(FlagsOption{
			WithoutType: true,
			NameDescOL:  true,
			Alignment:   AlignLeft,
			TagName:     FlagTagName,
		}),
	}
	// init commandBase
	app.commandBase = newCommandBase()

	if len(fn) > 0 {
		fn[0](app)
	}

	return app
}

// Config the application.
// Notice: must be called before adding a command
func (app *App) Config(fn func(a *App)) {
	if fn != nil {
		fn(app)
	}
}

// Exit get the app GlobalFlags
func (app *App) Exit(code int) {
	if app.ExitFunc == nil {
		os.Exit(code)
	}

	app.ExitFunc(code)
}

// binding global options
func (app *App) bindingGlobalOpts() {
	Logf(VerbDebug, "will begin binding global options")
	// global options flag
	// gf := flag.NewFlagSet(app.Args[0], flag.ContinueOnError)
	gf := app.GlobalFlags()

	// binding global options
	bindingCommonGOpts(gf)
	// add more ...
	gf.BoolOpt(&gOpts.showVer, "version", "V", false, "Display app version information")
	// This is a internal command
	gf.BoolVar(&gOpts.inCompletion, FlagMeta{
		Name: "cmd-completion",
		Desc: "generate completion scripts for bash/zsh",
		// hidden it
		Hidden: true,
	})

	// support binding custom global options
	if app.GOptsBinder != nil {
		app.GOptsBinder(gf)
	}
}

// initialize application
func (app *App) initialize() {
	app.names = make(map[string]int)

	// init some help tpl vars
	app.core.AddVars(app.core.innerHelpVars())

	// binding GlobalOpts
	app.bindingGlobalOpts()
	// parseGlobalOpts()

	// add default error handler.
	app.core.AddOn(EvtAppError, defaultErrHandler)

	app.fireEvent(EvtAppInit, nil)
	app.initialized = true
}

// SetLogo text and color style
func (app *App) SetLogo(logo string, style ...string) {
	app.Logo.Text = logo
	if len(style) > 0 {
		app.Logo.Style = style[0]
	}
}

// NewCommand create a new command
func (app *App) NewCommand(name, useFor string, config func(c *Command)) *Command {
	return NewCommand(name, useFor, config)
}

// Add add one or multi command(s)
func (app *App) Add(c *Command, more ...*Command) {
	app.AddCommand(c)

	// if has more command
	if len(more) > 0 {
		for _, cmd := range more {
			app.AddCommand(cmd)
		}
	}
}

// AddCommand add a new command to the app
func (app *App) AddCommand(c *Command) {
	// initialize application
	if !app.initialized {
		app.initialize()
	}

	// init command
	c.app = app
	// inherit global flags from application
	c.core.gFlags = app.gFlags

	// do add
	app.commandBase.addCommand(c)
}

// AddCommander to the application
func (app *App) AddCommander(cmder Commander) {
	c := cmder.Creator()
	c.Func = cmder.Execute

	// binding flags
	cmder.Config(c)
	app.AddCommand(c)
}

// ResolveName get real command name by alias
func (app *App) ResolveName(alias string) string {
	if name, has := app.aliases[alias]; has {
		return name
	}

	return alias
}

// RemoveCommand from the application
func (app *App) RemoveCommand(names ...string) int {
	var num int
	for _, name := range names {
		if app.removeCommand(name) {
			num++
		}
	}
	return num
}

func (app *App) removeCommand(name string) bool {
	if !app.IsCommand(name) {
		return false
	}

	// remove all aliases
	for alias, cName := range app.aliases {
		if cName == name {
			delete(app.aliases, alias)
		}
	}

	delete(app.names, name)
	delete(app.commands, name)
	return true
}

// IsAlias name check
func (app *App) IsAlias(str string) bool {
	_, has := app.aliases[str]
	return has
}

// AddAliases add alias names for a command
func (app *App) AddAliases(command string, aliases ...string) {
	app.addAliases(command, aliases, true)
}

// addAliases add alias names for a command
func (app *App) addAliases(command string, aliases []string, sync bool) {
	if app.aliases == nil {
		app.aliases = make(map[string]string)
	}

	c, has := app.commands[command]
	if !has {
		panicf("The command '%s' is not exists", command)
	}

	// add alias
	for _, alias := range aliases {
		if _, has := app.names[alias]; has {
			panicf("The name '%s' has been used as an command name", alias)
		}

		if cmd, has := app.aliases[alias]; has {
			panicf("The alias '%s' has been used by command '%s'", alias, cmd)
		}

		app.aliases[alias] = command
		// sync to Command
		if sync {
			c.Aliases = append(c.Aliases, alias)
		}
	}
}

// On add hook handler for a hook event
// func (app *App) BeforeInit(name string, handler HookFunc) {}

// On add hook handler for a hook event
func (app *App) On(name string, handler HookFunc) {
	Logf(VerbDebug, "add application hook: %s", name)

	app.core.On(name, handler)
}

func (app *App) fireEvent(event string, data interface{}) {
	Logf(VerbDebug, "trigger the application event: <mga>%s</>", event)

	app.core.Fire(event, app, data)
}

// stop application and exit
// func stop(code int) {
// 	os.Exit(code)
// }

// Names get all command names
func (app *App) Names() map[string]int {
	return app.names
}

// Commands get all commands
func (app *App) Commands() map[string]*Command {
	return app.commands
}

// CleanArgs get clean args
func (app *App) CleanArgs() []string {
	return app.cleanArgs
}
