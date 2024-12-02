package snaps

type Config struct {
	filename       string
	snapsDir       string
	extension      string
	update         *bool
	sortProperties bool
}

func defaultConfig() *Config { return &Config{snapsDir: "__snapshots__"} }

func (c *Config) Filename() string { return c.filename }

func (c *Config) SnapsDir() string { return c.snapsDir }

func (c *Config) Extension() string { return c.extension }

func (c *Config) Update() *bool { return c.update }

func (c *Config) SortProperties() bool { return c.sortProperties }

// WithConfig Create snaps with configuration
//
//	snaps.WithConfig(snaps.Filename("my_test")).MatchSnapshot(t, "hello world")
func WithConfig(args ...func(*Config)) *Config {
	s := defaultConfig()

	for _, arg := range args {
		arg(s)
	}

	return s
}

// Update determines whether to update snapshots or not
//
// It respects if running on CI.
func Update(u bool) func(*Config) { return func(c *Config) { c.update = &u } }

// Filename Specify folder name where snapshots are stored
//
//	default: __snapshots__
//
// this doesn't change the file extension see `snap.Ext`
func Filename(name string) func(*Config) { return func(c *Config) { c.filename = name } }

// Dir Specify folder name where snapshots are stored
//
//	default: __snapshots__
//
// Accepts absolute paths
func Dir(dir string) func(*Config) { return func(c *Config) { c.snapsDir = dir } }

// Ext Specify file name extension
//
// default: .snap
//
// Note: even if you specify a different extension the file still contain .snap
// e.g. if you specify .txt the file will be .snap.txt
func Ext(ext string) func(*Config) { return func(c *Config) { c.extension = ext } }

// SortProperties sort properties in snapshots
//
// default: false
func SortProperties() func(*Config) { return func(c *Config) { c.sortProperties = true } }
