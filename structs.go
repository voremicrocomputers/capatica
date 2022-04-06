package main

type Image struct {
	RealName string
	Sus      bool
}

type Capatica struct {
	RequestID  string
	ImagesSent [9]*Image
}

type MainRoutineRequest struct {
	DemandType        Demand
	Data              interface{}
	AssociatedRequest string
	ResponseChan      chan interface{}
}
