import CoreGraphics
import Foundation
// keystate.swift: prints which keys the OS believes are held right now (rows H2 and K6 of 21-12).
// Built on the host, run ONLY inside a Tart guest (p21-rig.sh keystate); nothing here posts input.
// Output: "keystate held=<names or none>" over the keys a Cmd chord or the release chord can leave down.
let keys: [(String, CGKeyCode)] = [("Cmd", 55), ("RCmd", 54), ("Shift", 56), ("Option", 58), ("Control", 59), ("KeyC", 8), ("KeyV", 9), ("KeyW", 13), ("KeyQ", 12), ("KeyA", 0), ("Return", 36)]
let held = keys.filter { CGEventSource.keyState(.combinedSessionState, key: $0.1) }.map { $0.0 }
print("keystate held=" + (held.isEmpty ? "none" : held.joined(separator: ",")))
