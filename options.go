package botcomm

// Opts configures startup. Nil slices, an empty InputMode and nil
// OnlineStatusVisible leave the corresponding server configuration unchanged.
// Non-nil empty slices clear the corresponding definitions.
type Opts struct {
	Commands            []*Command
	Settings            []SettingField
	Collections         []CollectionField
	InputMode           string
	OnlineStatusVisible *bool
	ArgumentCommands    bool
	NewSession          func(session *Session)
}

// SetCommands sets the command definitions to publish before connecting.
func (o *Opts) SetCommands(commands []*Command) *Opts {
	o.Commands = commands

	return o
}

// SetSettings sets the per-session settings schema to publish before connecting.
func (o *Opts) SetSettings(fields []SettingField) *Opts {
	o.Settings = fields

	return o
}

// SetCollections sets the session collection schema to publish before connecting.
func (o *Opts) SetCollections(fields []CollectionField) *Opts {
	o.Collections = fields

	return o
}

// SetInputMode sets the startup input mode: "messages", "commands" or "none".
func (o *Opts) SetInputMode(mode string) *Opts {
	o.InputMode = mode

	return o
}

// SetOnlineStatusVisible sets whether apps show the bot's connection status.
func (o *Opts) SetOnlineStatusVisible(visible bool) *Opts {
	o.OnlineStatusVisible = &visible

	return o
}

// SetArgumentCommands enables or disables routing of declared command input.
func (o *Opts) SetArgumentCommands(enabled bool) *Opts {
	o.ArgumentCommands = enabled

	return o
}

// SetNewSession sets the callback invoked when a user opens a new session.
func (o *Opts) SetNewSession(callback func(session *Session)) *Opts {
	o.NewSession = callback

	return o
}
