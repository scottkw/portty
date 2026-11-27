import AppKit
import ApplicationServices
// ax20 CMD TARGET [args]: Accessibility reads and presses for the Phase 20 VM rows. TARGET is a bundle
// id, a process name or pid:N. Built on the host with swiftc, copied into the guest by p20-rig.sh and run
// only there (the guest's Accessibility grant covers it; nothing here ever runs on the host).
//   text TARGET           the first text area's value, raw, then a line "chars=N"
//   windows TARGET        every window: title and frame
//   setframe TARGET X Y W H   move and size the first standard window
//   charrect TARGET LOC LEN   screen rect of a character range in the first text area
//   selrange TARGET       the first text area's selected range
//   scroll TARGET         the values of the first text area's scroll bars
//   find TARGET WORDS     every element whose role, title or description contains WORDS, with its frame
//   press TARGET WORDS    AXPress the first element find would list
//   front                 the frontmost application's name and pid
let a = CommandLine.arguments
guard a.count >= 2 else { print("usage: ax20 CMD TARGET ..."); exit(2) }

func attr(_ e: AXUIElement, _ n: String) -> AnyObject? {
  var v: AnyObject?
  AXUIElementCopyAttributeValue(e, n as CFString, &v)
  return v
}
func kids(_ e: AXUIElement) -> [AXUIElement] { (attr(e, kAXChildrenAttribute) as? [AXUIElement]) ?? [] }
func str(_ e: AXUIElement, _ n: String) -> String { (attr(e, n) as? String) ?? "" }
func point(_ e: AXUIElement) -> CGPoint {
  var p = CGPoint.zero
  if let v = attr(e, kAXPositionAttribute) { AXValueGetValue(v as! AXValue, .cgPoint, &p) }
  return p
}
func size(_ e: AXUIElement) -> CGSize {
  var s = CGSize.zero
  if let v = attr(e, kAXSizeAttribute) { AXValueGetValue(v as! AXValue, .cgSize, &s) }
  return s
}
func frame(_ e: AXUIElement) -> String {
  let p = point(e), s = size(e)
  return "@\(Int(p.x)),\(Int(p.y)) \(Int(s.width))x\(Int(s.height))"
}
func walk(_ e: AXUIElement, _ depth: Int, _ visit: (AXUIElement) -> Bool) -> Bool {
  if visit(e) { return true }
  if depth == 0 { return false }
  for k in kids(e) where walk(k, depth - 1, visit) { return true }
  return false
}
func all(_ e: AXUIElement, _ depth: Int, _ visit: (AXUIElement) -> Void) {
  _ = walk(e, depth) { visit($0); return false }
}

if a[1] == "cgwin" {
  // Every on-screen window at any layer, front to back: owner, layer, frame (titles need Screen Recording, not read here).
  let list = (CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]]) ?? []
  for w in list {
    let b = w[kCGWindowBounds as String] as? [String: Double] ?? [:]
    print("cgwin owner=\(w[kCGWindowOwnerName as String] ?? "?") pid=\(w[kCGWindowOwnerPID as String] ?? 0) layer=\(w[kCGWindowLayer as String] ?? 0) id=\(w[kCGWindowNumber as String] ?? 0) @\(Int(b["X"] ?? 0)),\(Int(b["Y"] ?? 0)) \(Int(b["Width"] ?? 0))x\(Int(b["Height"] ?? 0))")
  }
  exit(0)
}
if a[1] == "sysfocus" {
  // The same system-wide read the daemon's window gate makes (window_darwin.m pcFocusRead), from this process.
  var out: CFTypeRef?
  let e = AXUIElementCopyAttributeValue(AXUIElementCreateSystemWide(), kAXFocusedApplicationAttribute as CFString, &out)
  var pid: pid_t = 0
  if let o = out { AXUIElementGetPid(o as! AXUIElement, &pid) }
  print("sysfocus trusted=\(AXIsProcessTrusted()) err=\(e.rawValue) pid=\(pid)")
  // Alternatives, for the report: the frontmost app's own focused window, and the topmost layer-0 window's owner.
  if let f = NSWorkspace.shared.frontmostApplication {
    var w: CFTypeRef?
    let we = AXUIElementCopyAttributeValue(AXUIElementCreateApplication(f.processIdentifier), kAXFocusedWindowAttribute as CFString, &w)
    print("alt1 NSWorkspace front=\(f.localizedName ?? "?") pid=\(f.processIdentifier) its AXFocusedWindow err=\(we.rawValue)")
  }
  let wl = (CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]]) ?? []
  if let top = wl.first(where: { ($0[kCGWindowLayer as String] as? Int) == 0 }) {
    print("alt2 topmost layer-0 window owner=\(top[kCGWindowOwnerName as String] ?? "?") pid=\(top[kCGWindowOwnerPID as String] ?? 0)")
  }
  exit(0)
}
if a[1] == "front" {
  if let f = NSWorkspace.shared.frontmostApplication { print("front: \(f.localizedName ?? "?") pid=\(f.processIdentifier)") }
  exit(0)
}
guard a.count >= 3 else { print("usage: ax20 CMD TARGET ..."); exit(2) }
func pidOf(_ t: String) -> pid_t? {
  if t.hasPrefix("pid:") { return pid_t(t.dropFirst(4)) }
  if let r = NSRunningApplication.runningApplications(withBundleIdentifier: t).first { return r.processIdentifier }
  return NSWorkspace.shared.runningApplications.first { $0.localizedName == t }?.processIdentifier
}
guard let pid = pidOf(a[2]) else { print("target not running"); exit(1) }
let app = AXUIElementCreateApplication(pid)
AXUIElementSetMessagingTimeout(app, 2)
let windows = (attr(app, kAXWindowsAttribute) as? [AXUIElement]) ?? []
func textArea() -> AXUIElement? {
  var found: AXUIElement?
  for w in windows {
    _ = walk(w, 8) { e in
      if str(e, kAXRoleAttribute) == "AXTextArea" { found = e; return true }
      return false
    }
    if found != nil { break }
  }
  return found
}
func hit(_ e: AXUIElement, _ words: String) -> Bool {
  let hay = [str(e, kAXRoleAttribute), str(e, kAXTitleAttribute), str(e, kAXDescriptionAttribute), str(e, kAXRoleDescriptionAttribute)].joined(separator: " ").lowercased()
  return !words.isEmpty && hay.contains(words.lowercased())
}

switch a[1] {
case "text":
  guard let t = textArea() else { print("no text area"); exit(1) }
  let s = str(t, kAXValueAttribute)
  print(s, terminator: "")
  print("\nchars=\(s.count) utf8=\(s.utf8.count)")
case "windows":
  for w in windows { print("window title=[\(str(w, kAXTitleAttribute))] \(frame(w)) minimized=\((attr(w, kAXMinimizedAttribute) as? Bool) ?? false)") }
case "setframe":
  // An optional TITLE picks the window whose title contains it (the Fonts panel is not a standard window).
  let want = a.count == 8 ? a[7] : nil
  guard a.count == 7 || a.count == 8,
    let w = (want != nil ? windows.first(where: { str($0, kAXTitleAttribute).contains(want!) }) : (windows.first(where: { str($0, kAXSubroleAttribute) == "AXStandardWindow" }) ?? windows.first))
  else { print("usage: setframe T X Y W H [TITLE]"); exit(1) }
  var p = CGPoint(x: Double(a[3])!, y: Double(a[4])!), s = CGSize(width: Double(a[5])!, height: Double(a[6])!)
  AXUIElementSetAttributeValue(w, kAXPositionAttribute as CFString, AXValueCreate(.cgPoint, &p)!)
  AXUIElementSetAttributeValue(w, kAXSizeAttribute as CFString, AXValueCreate(.cgSize, &s)!)
  print("window \(frame(w))")
case "charrect":
  guard a.count == 5, let t = textArea() else { print("usage: charrect T LOC LEN (needs a text area)"); exit(1) }
  var r = CFRange(location: Int(a[3])!, length: Int(a[4])!)
  var out: CFTypeRef?
  let rv = AXValueCreate(.cfRange, &r)!
  let err = AXUIElementCopyParameterizedAttributeValue(t, "AXBoundsForRange" as CFString, rv, &out)
  var cg = CGRect.zero
  if err == .success, let o = out, AXValueGetValue(o as! AXValue, .cgRect, &cg) {
    print("charrect \(cg.origin.x) \(cg.origin.y) \(cg.size.width) \(cg.size.height)")
  } else { print("charrect error \(err.rawValue)"); exit(1) }
case "selrange":
  guard let t = textArea(), let v = attr(t, kAXSelectedTextRangeAttribute) else { print("no selection"); exit(1) }
  var r = CFRange()
  AXValueGetValue(v as! AXValue, .cfRange, &r)
  print("selrange loc=\(r.location) len=\(r.length)")
case "scroll":
  guard let t = textArea(), let sa = attr(t, kAXParentAttribute) else { print("no text area"); exit(1) }
  for k in kids(sa as! AXUIElement) where str(k, kAXRoleAttribute) == "AXScrollBar" {
    print("scrollbar orientation=\(str(k, kAXOrientationAttribute)) value=\(attr(k, kAXValueAttribute) ?? "?" as AnyObject)")
  }
case "find", "press":
  let words = a.count > 3 ? a[3...].joined(separator: " ") : ""
  var n = 0
  for w in windows {
    all(w, 9) { e in
      guard hit(e, words) else { return }
      n += 1
      // Only the value's length is printed: a password field's contents are never read out.
      print("hit role=\(str(e, kAXRoleAttribute)) title=[\(str(e, kAXTitleAttribute))] desc=[\(str(e, kAXDescriptionAttribute))] valueChars=\(str(e, kAXValueAttribute).count) \(frame(e))")
      if a[1] == "press" && n == 1 { print("press: \(AXUIElementPerformAction(e, kAXPressAction as CFString).rawValue)") }
    }
  }
  if n == 0 { print("no match"); exit(1) }
default:
  print("unknown command"); exit(2)
}
