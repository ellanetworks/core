package models

type TraceData struct {
	TraceRef                 string
	TraceDepth               TraceDepth
	NeTypeList               string
	EventList                string
	CollectionEntityIpv4Addr string
	CollectionEntityIpv6Addr string
	InterfaceList            string
}
