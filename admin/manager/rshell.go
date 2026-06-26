package manager

type rshellManager struct {
	RShellMessChan chan interface{}
	ReadyChan      chan bool
	ConnChan       chan uint64
	StopDoneChan   chan bool
}

func newRShellManager() *rshellManager {
	m := new(rshellManager)
	m.RShellMessChan = make(chan interface{}, 5)
	m.ReadyChan = make(chan bool, 1)
	m.ConnChan = make(chan uint64, 1)
	m.StopDoneChan = make(chan bool, 1)
	return m
}
