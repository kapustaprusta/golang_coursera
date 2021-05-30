package main

import (
	context "context"
	"encoding/json"
	"net"
	"regexp"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

type myService struct {
	Events      []chan *Event
	EventsMutex *sync.RWMutex
	Stats       []Stat
	StatsMutex  *sync.Mutex
	ACLRules    map[string][]*regexp.Regexp
	ACLMutex    *sync.Mutex
}

func newMyService(ACLRules map[string][]string) *myService {
	myService := &myService{
		Events:      make([]chan *Event, 0),
		EventsMutex: &sync.RWMutex{},
		Stats:       make([]Stat, 0),
		StatsMutex:  &sync.Mutex{},
		ACLRules:    make(map[string][]*regexp.Regexp),
		ACLMutex:    &sync.Mutex{},
	}

	for consumer, allowedMethods := range ACLRules {
		for _, allowedMethod := range allowedMethods {
			myService.ACLRules[consumer] = append(myService.ACLRules[consumer], regexp.MustCompile(allowedMethod))
		}
	}

	return myService
}

func (ms *myService) addEvent(event *Event) {
	ms.EventsMutex.Lock()
	for _, chanEvents := range ms.Events {
		chanEvents <- event
	}
	ms.EventsMutex.Unlock()
}

func (ms *myService) addByConsumer(consumer string) {
	ms.StatsMutex.Lock()
	for idx := 0; idx < len(ms.Stats); idx++ {
		if ms.Stats[idx].ByConsumer == nil {
			ms.Stats[idx].ByConsumer = make(map[string]uint64)
		}

		ms.Stats[idx].ByConsumer[consumer]++
	}
	ms.StatsMutex.Unlock()
}

func (ms *myService) addByMethod(method string) {
	ms.StatsMutex.Lock()
	for idx := 0; idx < len(ms.Stats); idx++ {
		if ms.Stats[idx].ByMethod == nil {
			ms.Stats[idx].ByMethod = make(map[string]uint64)
		}

		ms.Stats[idx].ByMethod[method]++
	}
	ms.StatsMutex.Unlock()
}

func (ms *myService) Check(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (ms *myService) Add(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (ms *myService) Test(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (ms *myService) Logging(nothing *Nothing, srv Admin_LoggingServer) error {
	ms.EventsMutex.Lock()
	currSrvEvents := make(chan *Event, 1000)
	ms.Events = append(ms.Events, currSrvEvents)
	ms.EventsMutex.Unlock()

	for {
		lastEvent := <-currSrvEvents
		err := srv.Send(lastEvent)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ms *myService) Statistics(statInterval *StatInterval, srv Admin_StatisticsServer) error {
	ms.StatsMutex.Lock()
	currSrvId := len(ms.Stats)
	ms.Stats = append(ms.Stats, Stat{})
	ms.StatsMutex.Unlock()
	srvTicker := time.NewTicker(time.Second * time.Duration(statInterval.GetIntervalSeconds()))

	for {
		<-srvTicker.C
		ms.StatsMutex.Lock()
		err := srv.Send(&ms.Stats[currSrvId])
		ms.Stats[currSrvId] = Stat{}
		ms.StatsMutex.Unlock()
		if err != nil {
			return err
		}
	}

	return nil
}

func (ms *myService) unaryAuthInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	consumer, isOk := md["consumer"]
	if !isOk {
		return nil, status.Errorf(codes.Unauthenticated, "")
	}

	ms.addEvent(&Event{
		Host:     "127.0.0.1:",
		Consumer: consumer[0],
		Method:   info.FullMethod,
	})

	ms.addByConsumer(consumer[0])
	ms.addByMethod(info.FullMethod)

	ms.ACLMutex.Lock()
	allowedMethodsRegExp := ms.ACLRules[consumer[0]]
	ms.ACLMutex.Unlock()

	isAllowed := false
	for _, allowedMethodRegExp := range allowedMethodsRegExp {
		if allowedMethodRegExp.MatchString(info.FullMethod) {
			isAllowed = true

			break
		}
	}

	if !isAllowed {
		return nil, status.Errorf(codes.Unauthenticated, "")
	}

	return handler(ctx, req)
}

func (ms *myService) streamAuthInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	consumer, isOk := md["consumer"]
	if !isOk {
		return status.Errorf(codes.Unauthenticated, "")
	}

	ms.addEvent(&Event{
		Host:     "127.0.0.1:",
		Consumer: consumer[0],
		Method:   info.FullMethod,
	})

	ms.addByConsumer(consumer[0])
	ms.addByMethod(info.FullMethod)

	ms.ACLMutex.Lock()
	allowedMethodsRegExp := ms.ACLRules[consumer[0]]
	ms.ACLMutex.Unlock()

	isAllowed := false
	for _, allowedMethodRegExp := range allowedMethodsRegExp {
		if allowedMethodRegExp.MatchString(info.FullMethod) {
			isAllowed = true

			break
		}
	}

	if !isAllowed {
		return status.Errorf(codes.Unauthenticated, "")
	}

	return handler(srv, ss)
}

func StartMyMicroservice(ctx context.Context, listenAddr string, ACLData string) error {
	ACLRules := make(map[string][]string)
	err := json.Unmarshal([]byte(ACLData), &ACLRules)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	myService := newMyService(ACLRules)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(myService.unaryAuthInterceptor),
		grpc.StreamInterceptor(myService.streamAuthInterceptor))

	RegisterBizServer(grpcServer, myService)
	RegisterAdminServer(grpcServer, myService)

	go func() {
		grpcServer.Serve(listener)
	}()

	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()

	return nil
}
