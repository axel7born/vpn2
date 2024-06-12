package pathcontroller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
)

type clientRouter struct {
	pinger    pinger
	netRouter netRouter

	log        logr.Logger
	checkedNet *net.IPNet
	primary    net.IP
	mu         sync.Mutex
	goodIPs    map[string]struct{}
	ticker     *time.Ticker
}

type netRouter interface {
	updateRouting(net.IP) error
}

type pinger interface {
	Ping(client net.IP) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []net.IP) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.ticker.C:
			r.pingAllShootClients(clientIPs)
			err := r.determinePrimaryShootClient()
			if err != nil {
				// dont return error here because in creation there will be some time when nothing is
				// available. If we returned the error we would exit the path-controller
				r.log.Error(err, "")
			}
		}
	}
}

func (r *clientRouter) selectNewPrimaryShootClient() (net.IP, error) {
	// just use a random ip that is in goodIps map
	for ip := range r.goodIPs {
		return net.ParseIP(ip), nil
	}
	return nil, errors.New("no more good ips in pool")
}

func (r *clientRouter) determinePrimaryShootClient() error {
	_, ok := r.goodIPs[r.primary.String()]
	if !ok {
		newIP, err := r.selectNewPrimaryShootClient()
		if err != nil {
			return fmt.Errorf("error selecting a new shoot client: %w", err)
		}
		err = r.netRouter.updateRouting(newIP)
		if err != nil {
			return err
		}
		r.log.Info("switching primary shoot client", "old", r.primary, "new", newIP)
		r.primary = newIP
	}
	return nil
}

func (r *clientRouter) pingAllShootClients(clients []net.IP) {
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.pinger.Ping(client)
			r.mu.Lock()
			defer r.mu.Unlock()
			if err != nil {
				r.log.Info("client not healthy, removing from pool", "ip", client)
				delete(r.goodIPs, client.String())
			} else {
				r.goodIPs[client.String()] = struct{}{}
			}
		}()
	}
	wg.Wait()
}

type netlinkRouter struct {
	podNetwork     *net.IPNet
	serviceNetwork *net.IPNet
	nodeNetwork    *net.IPNet

	log logr.Logger
}

func (r *netlinkRouter) updateRouting(newIP net.IP) error {
	bondDev, err := netlink.LinkByName("bond0")
	if err != nil {
		return err
	}

	nets := []*net.IPNet{
		r.serviceNetwork,
		r.podNetwork,
	}
	if r.nodeNetwork != nil {
		nets = append(nets, r.nodeNetwork)
	}

	for _, n := range nets {
		route := routeForNetwork(n, newIP, bondDev)
		r.log.Info("replacing route", "route", route, "net", n)
		err = netlink.RouteReplace(&route)
		if err != nil {
			return fmt.Errorf("error replacing route for %s: %w", n, err)
		}
	}
	return nil
}

func routeForNetwork(net *net.IPNet, newIP net.IP, bondLink netlink.Link) netlink.Route {
	// ip route replace $net via $newIp dev bond0
	return netlink.Route{
		// Gw is the equivalent to via in ip route replace command
		Gw:  newIP,
		Dst: net,
		//Table:     unix.RT_TABLE_MAIN,
		LinkIndex: bondLink.Attrs().Index,
	}
}
