import CoreGraphics
import Foundation
// key.swift: keyboard CGEvent poster for the Phase 21 key-routing probe. Built on the host, run ONLY
// inside the Tart VM (p21-rig.sh key2); nothing here may run on the host.
//
//   key CODE [cmd] [ctrl] [opt] [shift] [hold MS]
//       modifier-down events first (flags accumulate), key down, optional hold MS before the
//       key up, key up, modifier-up events in reverse order. CODE is a virtual key code.
//   winfo OWNER   print "x y w h" (points) of OWNER's largest on-screen window, or exit 1
//
// Posted at the HID tap with explicit flags, like the VM's click tool.
let a = CommandLine.arguments
guard a.count >= 3 else { FileHandle.standardError.write(Data("usage: key CODE [cmd|ctrl|opt|shift] [hold MS] | winfo OWNER\n".utf8)); exit(2) }

if a[1] == "winfo" {
  let list = (CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]]) ?? []
  var best: CGRect?
  for w in list where (w[kCGWindowOwnerName as String] as? String) == a[2] && (w[kCGWindowLayer as String] as? Int) == 0 {
    guard let b = w[kCGWindowBounds as String] as? [String: Double] else { continue }
    let r = CGRect(x: b["X"] ?? 0, y: b["Y"] ?? 0, width: b["Width"] ?? 0, height: b["Height"] ?? 0)
    if best == nil || r.width * r.height > best!.width * best!.height { best = r }
  }
  guard let r = best else { exit(1) }
  print(Int(r.origin.x), Int(r.origin.y), Int(r.width), Int(r.height))
  exit(0)
}

guard a[1] == "key", let code = Int(a[2]) else { FileHandle.standardError.write(Data("key: bad code\n".utf8)); exit(2) }
let src = CGEventSource(stateID: .hidSystemState)
var flags: CGEventFlags = []
var mods: [(CGKeyCode, CGEventFlags)] = []
var holdMS: UInt32 = 0
var i = 3
while i < a.count {
  switch a[i] {
  case "cmd": mods.append((55, .maskCommand))
  case "ctrl": mods.append((59, .maskControl))
  case "opt": mods.append((58, .maskAlternate))
  case "shift": mods.append((56, .maskShift))
  case "hold":
    i += 1
    guard i < a.count, let ms = UInt32(a[i]) else { FileHandle.standardError.write(Data("key: hold needs MS\n".utf8)); exit(2) }
    holdMS = ms
  default: FileHandle.standardError.write(Data("key: unknown arg \(a[i])\n".utf8)); exit(2)
  }
  i += 1
}
func post(_ k: CGKeyCode, _ down: Bool, _ f: CGEventFlags) {
  guard let e = CGEvent(keyboardEventSource: src, virtualKey: k, keyDown: down) else { FileHandle.standardError.write(Data("key: CGEvent nil\n".utf8)); exit(1) }
  e.flags = f
  e.post(tap: .cghidEventTap)
}
for (k, f) in mods { flags.insert(f); post(k, true, flags); usleep(40000) }
post(CGKeyCode(code), true, flags)
usleep(holdMS > 0 ? holdMS * 1000 : 50000)
post(CGKeyCode(code), false, flags)
usleep(50000)
for (k, f) in mods.reversed() { flags.remove(f); post(k, false, flags); usleep(20000) }
