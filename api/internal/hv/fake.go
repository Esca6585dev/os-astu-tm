package hv

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// Fake — Proxmox-syz ýerli synag üçin. Maşynlary we bekaplary ýatda saklaýar.
type Fake struct {
	mu      sync.Mutex
	next    int
	seq     int
	vms     map[int]*fakeVM
	backups map[string]int64 // volid → ölçeg
	Delay   time.Duration
}

type fakeVM struct {
	spec    VMSpec
	running bool
	started time.Time
}

func NewFake() *Fake {
	return &Fake{next: 1000, vms: map[int]*fakeVM{}, backups: map[string]int64{}, Delay: 2 * time.Second}
}

func (f *Fake) sleep(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(f.Delay):
		return nil
	}
}

func (f *Fake) NextVMID(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		f.next++
		if _, ok := f.vms[f.next]; !ok {
			return f.next, nil
		}
	}
}

func (f *Fake) Create(ctx context.Context, s VMSpec) error {
	if err := f.sleep(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vms[s.VMID] = &fakeVM{spec: s, running: true, started: time.Now()}
	return nil
}

// vm maşyny gaýtarýar; ýok bolsa döredýär (panel öňden bar bolan maşynlary hem dolandyryp biler ýaly).
// Çagyryjy f.mu-ny saklamaly.
func (f *Fake) vm(vmid int) *fakeVM {
	vm, ok := f.vms[vmid]
	if !ok {
		vm = &fakeVM{}
		f.vms[vmid] = vm
	}
	return vm
}

func (f *Fake) set(vmid int, running bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	vm := f.vm(vmid)
	vm.running = running
	if running {
		vm.started = time.Now()
	}
	return nil
}

func (f *Fake) Start(ctx context.Context, _ string, vmid int) error {
	_ = f.sleep(ctx)
	return f.set(vmid, true)
}
func (f *Fake) Stop(ctx context.Context, _ string, vmid int) error {
	_ = f.sleep(ctx)
	return f.set(vmid, false)
}
func (f *Fake) Reboot(ctx context.Context, _ string, vmid int) error {
	_ = f.sleep(ctx)
	return f.set(vmid, true)
}
func (f *Fake) SetPassword(ctx context.Context, _ string, _ int, _, _ string) error {
	return f.sleep(ctx)
}
func (f *Fake) Destroy(ctx context.Context, _ string, vmid int) error {
	_ = f.sleep(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.vms, vmid)
	return nil
}

func (f *Fake) Usage(_ context.Context, _ string, vmid int) (Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	vm, ok := f.vms[vmid]
	if !ok {
		return Usage{}, ErrNotFound
	}
	u := Usage{Status: "stopped", MemMax: int64(vm.spec.MemoryMB) << 20, DiskMax: int64(vm.spec.DiskGB) << 30}
	if vm.running {
		u.Status = "running"
		u.CPU = 0.05 + rand.Float64()*0.3
		u.MemUsed = int64(float64(u.MemMax) * (0.4 + rand.Float64()*0.3))
		u.UptimeSec = int64(time.Since(vm.started).Seconds())
		u.NetInBytes = u.UptimeSec * 12000
		u.NetOutBytes = u.UptimeSec * 30000
	}
	return u, nil
}

func (f *Fake) SetFirewall(ctx context.Context, _ string, vmid int, _ string, fw Firewall) error {
	if err := f.sleep(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vm(vmid).spec.Firewall = fw
	return nil
}

func (f *Fake) Backup(ctx context.Context, _ string, vmid int, storage, _ string) (BackupInfo, error) {
	if err := f.sleep(ctx); err != nil {
		return BackupInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	disk := int64(f.vm(vmid).spec.DiskGB)
	if disk == 0 {
		disk = 20
	}
	// zstd bilen gysylan disk: doly göwrümiň takmynan 4–10%-i
	size := int64(float64(disk<<30) * (0.04 + rand.Float64()*0.06))
	volid := fmt.Sprintf("%s:backup/vzdump-qemu-%d-%s-%d.vma.zst", storage, vmid, time.Now().Format("2006_01_02-15_04_05"), f.seq)
	f.backups[volid] = size
	return BackupInfo{Volid: volid, Size: size}, nil
}

func (f *Fake) RestoreBackup(ctx context.Context, _ string, vmid int, volid, _ string) error {
	if err := f.sleep(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.backups[volid]; !ok {
		return errors.New("bekap faýly tapylmady: " + volid)
	}
	f.vm(vmid).running = false
	return nil
}

func (f *Fake) DeleteBackup(ctx context.Context, _ string, volid string) error {
	if err := f.sleep(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.backups, volid)
	return nil
}
