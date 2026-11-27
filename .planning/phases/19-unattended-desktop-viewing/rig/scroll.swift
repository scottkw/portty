import CoreGraphics
import Foundation
// scroll X Y DY: move the pointer to X,Y (points) and post one pixel-unit scroll-wheel event of DY
// lines (negative scrolls the content up, i.e. shows what is below). Built on the host with swiftc,
// copied into the guest and run only there (19-15 F1: the Background service row sits below the
// Settings footer until the pane is scrolled).
let a = CommandLine.arguments
guard a.count == 4, let x = Double(a[1]), let y = Double(a[2]), let dy = Int32(a[3]) else {
  print("usage: scroll x y dy"); exit(2)
}
let p = CGPoint(x: x, y: y)
CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: p, mouseButton: .left)?.post(tap: .cghidEventTap)
usleep(100_000)
CGEvent(scrollWheelEvent2Source: nil, units: .pixel, wheelCount: 1, wheel1: dy, wheel2: 0, wheel3: 0)?.post(tap: .cghidEventTap)
