package cocaine

import (
	"time"

	"github.com/ugorji/go/codec"
)

type ServiceResult interface {
	Extract(interface{}) error
	Err() error
}

type serviceRes struct {
	res []byte
	err error
}

//Unpacks the result of the called method in the passed structure.
//You can transfer the structure of a particular type that will avoid the type checking. Look at examples.
func (s *serviceRes) Extract(target interface{}) (err error) {
	err = codec.NewDecoderBytes(s.res, h).Decode(&target)
	return
}

//Error status
func (s *serviceRes) Err() error {
	return s.err
}

//
type ServiceError struct {
	Code    int
	Message string
}

func (err *ServiceError) Error() string {
	return err.Message
}

func getServiceChanPair(stop <-chan bool) (In chan ServiceResult, Out chan ServiceResult) {
	In = make(chan ServiceResult)
	Out = make(chan ServiceResult)
	finished := false
	go func() {
		var pending []ServiceResult
		for {
			var out chan ServiceResult
			var first ServiceResult

			if len(pending) > 0 {
				first = pending[0]
				out = Out
			} else if finished {
				close(Out)
				break
			}

			select {
			case incoming, ok := <-In:
				if ok {
					pending = append(pending, incoming)
				} else {
					finished = true
					In = nil
				}

			case out <- first:
				pending = pending[1:]

			case <-stop: // Notification from Close()
				return
			}
		}
	}()
	return
}

//Allows you to invoke methods of services and send events to other cloud applications.
type Service struct {
	sessions *keeperStruct
	unpacker *streamUnpacker
	stop     chan bool
	ResolveResult
	socketIO
}

//Creates new service instance with specifed name.
//Optional parameter is a network endpoint of the locator (default ":10053"). Look at Locator.
func NewService(name string, args ...interface{}) (s *Service, err error) {
	l, err := NewLocator(args...)
	if err != nil {
		return
	}
	defer l.Close()
	info := <-l.Resolve(name)
	sock, err := newAsyncRWSocket("tcp", info.Endpoint.AsString(), time.Second*5)
	if err != nil {
		return
	}
	s = &Service{
		sessions:      newKeeperStruct(),
		unpacker:      newStreamUnpacker(),
		stop:          make(chan bool),
		ResolveResult: info,
		socketIO:      sock,
	}
	go s.loop()
	return
}

func (service *Service) loop() {
	for data := range service.socketIO.Read() {
		for _, item := range service.unpacker.Feed(data) {
			switch msg := item.(type) {
			case *chunk:
				service.sessions.Get(msg.getSessionID()) <- &serviceRes{msg.Data, nil}
			case *choke:
				close(service.sessions.Get(msg.getSessionID()))
				service.sessions.Detach(msg.getSessionID())
			case *errorMsg:
				service.sessions.Get(msg.getSessionID()) <- &serviceRes{nil, &ServiceError{msg.Code, msg.Message}}
			}
		}
	}
}

//Calls a remote method by name and pass args
func (service *Service) Call(name string, args ...interface{}) chan ServiceResult {
	method, err := service.getMethodNumber(name)
	if err != nil {
		errorOut := make(chan ServiceResult, 1)
		errorOut <- &serviceRes{nil, &ServiceError{-100, "Wrong method name"}}
		return errorOut
	}
	in, out := getServiceChanPair(service.stop)
	id := service.sessions.Attach(in)
	msg := ServiceMethod{messageInfo{method, id}, args}
	service.socketIO.Write() <- packMsg(&msg)
	return out
}

//Disposes resources of a service. You must call this method if the service isn't used anymore.
func (service *Service) Close() {
	close(service.stop) // Broadcast all related goroutines about disposing
	service.socketIO.Close()
}
