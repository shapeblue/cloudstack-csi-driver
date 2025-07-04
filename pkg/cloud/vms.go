package cloud

import (
	"context"

	"k8s.io/klog/v2"
)

func (c *client) GetVMByID(ctx context.Context, vmID string) (*VM, error) {
	logger := klog.FromContext(ctx)
	p := c.VirtualMachine.NewListVirtualMachinesParams()
	p.SetId(vmID)
	if c.projectID != "" {
		p.SetProjectid(c.projectID)
	}
	logger.V(2).Info("CloudStack API call", "command", "ListVirtualMachines", "params", map[string]string{
		"id":        vmID,
		"projectID": c.projectID,
	})
	l, err := c.VirtualMachine.ListVirtualMachines(p)
	if err != nil {
		return nil, err
	}
	if l.Count == 0 {
		return nil, ErrNotFound
	}
	if l.Count > 1 {
		return nil, ErrTooManyResults
	}
	vm := l.VirtualMachines[0]
	logger.V(2).Info("Returning VM", "vmID", vm.Id, "zoneID", vm.Zoneid)
	return &VM{
		ID:     vm.Id,
		ZoneID: vm.Zoneid,
	}, nil
}

func (c *client) getVMByName(ctx context.Context, name string) (*VM, error) {
	logger := klog.FromContext(ctx)
	p := c.VirtualMachine.NewListVirtualMachinesParams()
	p.SetName(name)
	logger.V(2).Info("CloudStack API call", "command", "ListVirtualMachines", "params", map[string]string{
		"name": name,
	})
	l, err := c.VirtualMachine.ListVirtualMachines(p)
	if err != nil {
		return nil, err
	}
	if l.Count == 0 {
		return nil, ErrNotFound
	}
	if l.Count > 1 {
		return nil, ErrTooManyResults
	}
	vm := l.VirtualMachines[0]

	return &VM{
		ID:     vm.Id,
		ZoneID: vm.Zoneid,
	}, nil
}
