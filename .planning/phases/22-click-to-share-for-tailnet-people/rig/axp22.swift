import AppKit
import ApplicationServices
// axp22 CMD BUNDLE ...: Accessibility reads and presses for the Phase 22 VM rows. Built on the host with
// swiftc (compile only) and run ONLY inside the guests, where the guest's own Accessibility grant covers it.
//   find BUNDLE WORDS            every element, at any depth, whose role, title, description or value contains
//                                WORDS: role, title, description, value (first 80 chars), AXEnabled, frame
//   press BUNDLE WORDS           AXPress the first such element that is a button, switch, checkbox or menu item
//                                (role AXButton, AXCheckBox, AXSwitch, AXRadioButton, AXMenuItem, AXPopUpButton)
//   has BUNDLE WORDS             exit 0 and print "yes" when something matches, else print "no" and exit 1
//   tray BUNDLE items            open the status item's menu, print each titled item with enabled, close it
//   tray BUNDLE press PREFIX     open the status item's menu and AXPress the first item whose title starts with PREFIX
//   tray BUNDLE icon             attributes of the status item itself (description, title, help, frame)
// Values of password-like fields are never printed: only the first 80 characters of a value, and only for
// roles that are not AXTextField/AXSecureTextField.
let a = CommandLine.arguments
guard a.count >= 3 else { print("usage: axp22 CMD BUNDLE ..."); exit(2) }

func attr(_ e: AXUIElement, _ n: String) -> AnyObject? {
  var v: AnyObject?
  AXUIElementCopyAttributeValue(e, n as CFString, &v)
  return v
}
func kids(_ e: AXUIElement) -> [AXUIElement] { (attr(e, kAXChildrenAttribute) as? [AXUIElement]) ?? [] }
func str(_ e: AXUIElement, _ n: String) -> String {
  if let s = attr(e, n) as? String { return s }
  if let n = attr(e, n) as? NSNumber { return n.stringValue }
  return ""
}
func frame(_ e: AXUIElement) -> String {
  var p = CGPoint.zero, s = CGSize.zero
  if let v = attr(e, kAXPositionAttribute) { AXValueGetValue(v as! AXValue, .cgPoint, &p) }
  if let v = attr(e, kAXSizeAttribute) { AXValueGetValue(v as! AXValue, .cgSize, &s) }
  return "@\(Int(p.x)),\(Int(p.y)) \(Int(s.width))x\(Int(s.height))"
}
func walk(_ e: AXUIElement, _ depth: Int, _ visit: (AXUIElement) -> Void) {
  visit(e)
  if depth == 0 { return }
  for k in kids(e) { walk(k, depth - 1, visit) }
}
func valueText(_ e: AXUIElement) -> String {
  let role = str(e, kAXRoleAttribute)
  if role == "AXSecureTextField" || role == "AXTextField" { return "chars=\(str(e, kAXValueAttribute).count)" }
  return String(str(e, kAXValueAttribute).prefix(80))
}
func hit(_ e: AXUIElement, _ w: String) -> Bool {
  str(e, kAXRoleAttribute).contains(w) || str(e, kAXTitleAttribute).contains(w) ||
    str(e, kAXDescriptionAttribute).contains(w) || valueText(e).contains(w)
}
let pressable: Set<String> = ["AXButton", "AXCheckBox", "AXSwitch", "AXRadioButton", "AXMenuItem", "AXPopUpButton"]

// Portty's GUI and its daemon share one bundle id; the process that owns windows or a status item is the one to read.
let candidates = NSRunningApplication.runningApplications(withBundleIdentifier: a[2])
guard let first = candidates.first else {
  print("app not running"); exit(1)
}
let run = candidates.first { c in
  let e = AXUIElementCreateApplication(c.processIdentifier)
  AXUIElementSetMessagingTimeout(e, 2)
  return attr(e, "AXExtrasMenuBar") != nil || !((attr(e, kAXWindowsAttribute) as? [AXUIElement]) ?? []).isEmpty
} ?? first
let app = AXUIElementCreateApplication(run.processIdentifier)
AXUIElementSetMessagingTimeout(app, 3)

if a[1] == "tray" {
  guard a.count >= 4 else { print("usage: axp22 tray BUNDLE items|press PREFIX|icon"); exit(2) }
  guard let extras = attr(app, "AXExtrasMenuBar") else { print("no status items"); exit(1) }
  let items = kids(extras as! AXUIElement)
  print("status items:", items.count)
  for item in items {
    if a[3] == "icon" {
      print("status-item title=[\(str(item, kAXTitleAttribute))] desc=[\(str(item, kAXDescriptionAttribute))] help=[\(str(item, kAXHelpAttribute))] id=[\(str(item, "AXIdentifier"))] \(frame(item))")
      continue
    }
    AXUIElementPerformAction(item, kAXPressAction as CFString)
    usleep(500_000)
    guard let menu = kids(item).first else { print("status item has no menu"); continue }
    var pressed = false
    for mi in kids(menu) {
      let t = str(mi, kAXTitleAttribute)
      if t.isEmpty { continue }
      let en = (attr(mi, kAXEnabledAttribute) as? Bool) ?? true
      if a[3] == "items" { print("tray-item: \(t) enabled=\(en ? "yes" : "no")") }
      if a[3] == "press", a.count >= 5, t.hasPrefix(a[4]), !pressed {
        pressed = true
        print("tray-press: \(t) rc=\(AXUIElementPerformAction(mi, kAXPressAction as CFString).rawValue)")
      }
    }
    if a[3] == "press" && !pressed { print("tray-press: no item starts with [\(a.count >= 5 ? a[4] : "")]") }
    if !(a[3] == "press" && pressed) { AXUIElementPerformAction(menu, "AXCancel" as CFString) }
    usleep(300_000)
  }
  exit(0)
}

let words = a.count > 3 ? a[3...].joined(separator: " ") : ""
let windows = (attr(app, kAXWindowsAttribute) as? [AXUIElement]) ?? []
var roots = windows
if let menubar = attr(app, kAXMenuBarAttribute) { roots.append(menubar as! AXUIElement) }
var n = 0
var pressedOne = false
for w in roots {
  walk(w, 40) { e in
    guard hit(e, words) else { return }
    n += 1
    if a[1] == "find" {
      print("hit role=\(str(e, kAXRoleAttribute)) title=[\(str(e, kAXTitleAttribute))] desc=[\(str(e, kAXDescriptionAttribute))] value=[\(valueText(e))] mark=[\(str(e, "AXMenuItemMarkChar"))] enabled=\(str(e, kAXEnabledAttribute)) \(frame(e))")
    }
    if a[1] == "press", !pressedOne, pressable.contains(str(e, kAXRoleAttribute)) {
      pressedOne = true
      print("press: role=\(str(e, kAXRoleAttribute)) title=[\(str(e, kAXTitleAttribute))] rc=\(AXUIElementPerformAction(e, kAXPressAction as CFString).rawValue)")
    }
  }
}
switch a[1] {
case "has": print(n > 0 ? "yes" : "no"); exit(n > 0 ? 0 : 1)
case "press": if !pressedOne { print("press: no pressable match for [\(words)] (\(n) hits)"); exit(1) }
default: if n == 0 { print("no match"); exit(1) }
}
