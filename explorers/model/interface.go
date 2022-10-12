package model

//go:generate mockgen -source=./interface.go -destination=../mock/BaseExplorerMock.go
type Isettings interface {
	GetLogPass(string) (log string, pass string)
	RAC_Path() string
	RAC_Port() string
	RAC_Host() string
	RAC_Login() string
	RAC_Pass() string
	GetExplorers() map[string]map[string]interface{}
	GetProperty(string, string, interface{}) interface{}
}

type IExplorers interface {
	StartExplore()
}

type Iexplorer interface {
	Start(IExplorers)
	Stop()
	Pause()
	Continue()
	StartExplore()
	GetName() string
}
