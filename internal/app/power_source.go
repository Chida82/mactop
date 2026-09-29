package app

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/ps/IOPowerSources.h>
#include <IOKit/ps/IOPSKeys.h>
#include <stdlib.h>
#include <string.h>

static int mactop_dict_number(CFDictionaryRef d, CFStringRef key, long long *out) {
    CFTypeRef v = CFDictionaryGetValue(d, key);
    if (!v || CFGetTypeID(v) != CFNumberGetTypeID())
        return 0;
    return CFNumberGetValue((CFNumberRef)v, kCFNumberLongLongType, out) ? 1 : 0;
}

static int mactop_dict_string(CFDictionaryRef d, CFStringRef key, char *out, int out_len) {
    CFTypeRef v = CFDictionaryGetValue(d, key);
    if (!v || CFGetTypeID(v) != CFStringGetTypeID() || out_len <= 0)
        return 0;
    out[0] = '\0';
    return CFStringGetCString((CFStringRef)v, out, out_len, kCFStringEncodingUTF8) ? 1 : 0;
}

static int mactop_read_source_type(char *out, int out_len) {
    if (!out || out_len <= 0)
        return 0;
    out[0] = '\0';
    CFTypeRef info = IOPSCopyPowerSourcesInfo();
    if (!info)
        return 0;
    CFStringRef providing = IOPSGetProvidingPowerSourceType(info);
    int ok = 0;
    if (providing) {
        ok = CFStringGetCString(providing, out, out_len, kCFStringEncodingUTF8) ? 1 : 0;
        CFRelease(providing);
    }
    CFRelease(info);
    return ok;
}

static int mactop_read_external_connected(void) {
    io_service_t bat = IOServiceGetMatchingService(kIOMainPortDefault,
                         IOServiceNameMatching("AppleSmartBattery"));
    if (!bat)
        return 0;
    CFTypeRef ext = IORegistryEntryCreateCFProperty(bat, CFSTR("ExternalConnected"),
                              kCFAllocatorDefault, 0);
    int connected = 0;
    if (ext && CFGetTypeID(ext) == CFBooleanGetTypeID())
        connected = CFBooleanGetValue((CFBooleanRef)ext) ? 1 : 0;
    if (ext)
        CFRelease(ext);
    IOObjectRelease(bat);
    return connected;
}

static void mactop_read_adapter(long long *watts, char *desc, int desc_len) {
    *watts = 0;
    if (desc && desc_len > 0)
        desc[0] = '\0';

    io_service_t bat = IOServiceGetMatchingService(kIOMainPortDefault,
                         IOServiceNameMatching("AppleSmartBattery"));
    if (!bat)
        return;

    CFTypeRef details = IORegistryEntryCreateCFProperty(bat, CFSTR("AdapterDetails"),
                              kCFAllocatorDefault, 0);
    if (details && CFGetTypeID(details) == CFDictionaryGetTypeID()) {
        long long w = 0;
        if (mactop_dict_number((CFDictionaryRef)details, CFSTR("Watts"), &w) && w > 0)
            *watts = w;
        mactop_dict_string((CFDictionaryRef)details, CFSTR("Description"), desc, desc_len);
    }
    if (details)
        CFRelease(details);
    IOObjectRelease(bat);
}
*/
import "C"

import (
	"strings"
	"sync"
	"time"
)

// PowerSupply describes where the machine draws power from and what the
// attached supply is rated for. Every field is read from the running system, so
// a laptop on a 30 W adapter, a laptop on a 140 W adapter, and a desktop whose
// supply is internal are each reported from their own hardware, with no
// per-model table.
type PowerSupply struct {
	// OnACPower is true when the system draws from an external supply rather
	// than its internal battery. It stays true on battery-less Macs.
	OnACPower bool `json:"on_ac_power" yaml:"on_ac_power" xml:"OnACPower" toon:"on_ac_power"`

	// AdapterConnected reports that an external supply is plugged in. It is
	// false on a desktop with an internal supply, and on a laptop unplugged.
	AdapterConnected bool `json:"adapter_connected" yaml:"adapter_connected" xml:"AdapterConnected" toon:"adapter_connected"`

	// AdapterWatts is the supply's rated output in watts, 0 when the hardware
	// publishes no rating. This is the figure
	// `system_profiler SPPowerDataType | grep Watt` prints.
	AdapterWatts int `json:"adapter_watts" yaml:"adapter_watts" xml:"AdapterWatts" toon:"adapter_watts"`

	// AdapterDescription names the supply, e.g. "pd charger". Empty when the
	// hardware does not name it.
	AdapterDescription string `json:"adapter_description,omitempty" yaml:"adapter_description,omitempty" xml:"AdapterDescription,omitempty" toon:"adapter_description"`

	// Source is IOKit's raw power-source type, e.g. "AC Power" or
	// "Battery Power". Empty when IOKit reports nothing.
	Source string `json:"source,omitempty" yaml:"source,omitempty" xml:"Source,omitempty" toon:"source"`
}

// Rated reports that an external supply is present and its rating is known,
// which is the condition for rendering a wattage figure.
func (p PowerSupply) Rated() bool {
	return p.AdapterConnected && p.AdapterWatts > 0
}

var (
	adapterMu    sync.Mutex
	adapterWatts int
	adapterDesc  string
	adapterKnown bool
)

func readAdapterDetails() (int, string) {
	var watts C.longlong
	desc := make([]C.char, 64)
	C.mactop_read_adapter(&watts, &desc[0], C.int(len(desc)))
	return int(watts), C.GoString(&desc[0])
}

func cachedAdapter() (int, string, bool) {
	adapterMu.Lock()
	defer adapterMu.Unlock()
	return adapterWatts, adapterDesc, adapterKnown
}

func cacheAdapter(watts int, desc string) {
	adapterMu.Lock()
	defer adapterMu.Unlock()
	adapterWatts, adapterDesc, adapterKnown = watts, desc, true
}

var (
	powerSupplyMu     sync.Mutex
	powerSupplyCached PowerSupply
	powerSupplyAt     time.Time
	powerSupplyTTL    = 5 * time.Second
)

func readPowerSupplyUncached() PowerSupply {
	sourceBuf := make([]C.char, 64)
	source := ""
	if C.mactop_read_source_type(&sourceBuf[0], C.int(len(sourceBuf))) != 0 {
		source = C.GoString(&sourceBuf[0])
	}
	connected := C.mactop_read_external_connected() != 0

	watts, desc, known := cachedAdapter()
	if !known && connected {
		watts, desc = readAdapterDetails()
		if watts > 0 || desc != "" {
			cacheAdapter(watts, desc)
		}
	}

	supply := PowerSupply{
		OnACPower:          strings.EqualFold(source, "AC Power"),
		AdapterConnected:   connected,
		AdapterWatts:       watts,
		AdapterDescription: desc,
		Source:             source,
	}
	if !supply.OnACPower && supply.AdapterConnected {
		supply.OnACPower = true
	}
	return supply
}

// GetPowerSupply caches the whole reading. Each IOKit round trip costs ~730 us
// (measured, p99 1.4 ms), and the render path calls this twice per sample under
// renderMutex, where the drain is a non-blocking select that silently drops a
// sample it cannot keep up with. Caching keeps that cost off the frame budget.
// The TTL has to exceed the default 1 s update interval, or the cache would
// still be re-read on every single tick.
func GetPowerSupply() PowerSupply {
	powerSupplyMu.Lock()
	defer powerSupplyMu.Unlock()
	if !powerSupplyAt.IsZero() && time.Since(powerSupplyAt) < powerSupplyTTL {
		return powerSupplyCached
	}
	powerSupplyCached = readPowerSupplyUncached()
	powerSupplyAt = time.Now()
	return powerSupplyCached
}
