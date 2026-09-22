package globals

type RequestTimeout struct {
	ConnectionTimeout int32 `name:"connect-timeout" help:"Set AWS Request ConnectionTimeout(Sec)" default:"60"`
	ReadTimeout       int32 `name:"read-timeout" help:"Set AWS Request ReadTimeout(Sec)" default:"120"`
}

type AWSCLIFlag struct {
	RequestTimeout
	AccessKey    string `name:"accessKey" help:"Set AWS AccessKey" default:""`
	SecretKey    string `name:"secretKey" help:"Set AWS SecretKey" default:""`
	SessionToken string `name:"sessionToken" help:"Set AWS SessionToken" default:""`
	Region       string `name:"region" short:"r" help:"Set AWS Region" default:""`
	Profile      string `name:"profile" short:"p" help:"Set AWS profile" default:""`
	UseEnv       bool   `name:"env" short:"e" help:"Allow envionment variables if set | Default is true" negatable:"" default:"1"`
}
