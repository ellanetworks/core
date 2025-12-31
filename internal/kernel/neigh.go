package kernel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"

	"github.com/vishvananda/netlink"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/kernel")

// AddNeighbour adds the provided IP as a neighbour
// on all links that have an address in the same subnet.
func AddNeighbour(ctx context.Context, neigh net.IP) error {
	_, span := tracer.Start(ctx, "Kernel Add Neighbour",
		trace.WithAttributes(
			attribute.String("IP", neigh.String()),
		))
	defer span.End()

	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("could not list network links: %v", err)
	}

	added := false

	for _, l := range links {
		addrs, err := netlink.AddrList(l, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("could not list addresses for link: %v", err)
		}

		for _, a := range addrs {
			if a.Contains(neigh) {
				err = addNeighbourForLink(neigh, l)
				if err != nil {
					return fmt.Errorf("could not add neighbour for link: %v", err)
				}

				added = true
			}
		}
	}

	if !added {
		return fmt.Errorf("could not add neighbour")
	}

	return nil
}

func addNeighbourForLink(neigh net.IP, link netlink.Link) error {
	nlNeigh := netlink.Neigh{
		LinkIndex: link.Attrs().Index,
		IP:        neigh,
		Flags:     netlink.NTF_EXT_MANAGED,
	}

	if err := netlink.NeighAdd(&nlNeigh); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}

	return nil
}
