import Foundation
import IOKit
import IOKit.hidsystem
import CoreGraphics

// capsset on|off|read: sets or reads the HID system's Caps Lock state (a synthetic Caps Lock key press does not
// set it). Compiled on the host (compile only) and run inside portty-test.
let a = CommandLine.arguments
let h = NXOpenEventStatus()
if h == 0 { print("capsset: could not open the event status"); exit(1) }
func read() -> Bool { CGEventSource.flagsState(.hidSystemState).contains(.maskAlphaShift) }
switch a.count > 1 ? a[1] : "" {
case "on": IOHIDSetModifierLockState(h, Int32(kIOHIDCapsLockState), true)
case "off": IOHIDSetModifierLockState(h, Int32(kIOHIDCapsLockState), false)
case "read": break
default: print("usage: capsset on|off|read"); exit(2)
}
usleep(200_000)
print("capsset: caps lock flag now \(read() ? "on" : "off")")
