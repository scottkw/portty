import AppKit
import ApplicationServices
// axswitch.swift: read, and press with Accessibility, the switch that follows a label in an app's windows
// (System Settings > Local Network lists "Portty" then its switch). Built on the host with swiftc, copied
// into a guest by p21-rig.sh and run only there; nothing here ever runs on the host.
//   axswitch BUNDLE            every switch with the static text just before it, and its value
//   axswitch BUNDLE LABEL      the same, only for LABEL
//   axswitch BUNDLE LABEL press   AXPress that switch, then read its value again
let a = CommandLine.arguments
guard a.count >= 2 else { print("usage: axswitch BUNDLE [LABEL [press]]"); exit(2) }
guard let app = NSRunningApplication.runningApplications(withBundleIdentifier: a[1]).first else { print("not running"); exit(1) }
func attr(_ e: AXUIElement, _ n: String) -> AnyObject? { var v: AnyObject?; AXUIElementCopyAttributeValue(e, n as CFString, &v); return v }
func str(_ e: AXUIElement, _ n: String) -> String { (attr(e, n) as? String) ?? "" }
func kids(_ e: AXUIElement) -> [AXUIElement] { (attr(e, kAXChildrenAttribute) as? [AXUIElement]) ?? [] }
let root = AXUIElementCreateApplication(app.processIdentifier)
AXUIElementSetMessagingTimeout(root, 2)
var label = ""
var found: [(String, AXUIElement)] = []
func walk(_ e: AXUIElement, _ depth: Int) {
  let role = str(e, kAXRoleAttribute)
  if role == "AXStaticText" { label = str(e, kAXValueAttribute) }
  if role == "AXCheckBox" && str(e, kAXSubroleAttribute) == "AXSwitch" { found.append((label, e)) }
  if depth > 0 { for k in kids(e) { walk(k, depth - 1) } }
}
for w in (attr(root, kAXWindowsAttribute) as? [AXUIElement]) ?? [] { walk(w, 40) }
func value(_ e: AXUIElement) -> String { "\(attr(e, kAXValueAttribute) ?? "?" as AnyObject)" }
let want = a.count > 2 ? a[2] : nil
let hits = found.filter { want == nil || $0.0 == want! }
if hits.isEmpty { print("no switch"); exit(1) }
for (l, e) in hits { print("switch label=[\(l)] value=\(value(e))") }
if a.count > 3 && a[3] == "press" {
  let r = AXUIElementPerformAction(hits[0].1, kAXPressAction as CFString)
  usleep(500_000)
  print("press: \(r.rawValue) value now=\(value(hits[0].1))")
}
