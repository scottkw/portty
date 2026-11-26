import AppKit
import ApplicationServices
// axtray BUNDLE_ID: open the app's status item menu through Accessibility, print every titled
// item as "tray-item: TITLE enabled=yes|no", close the menu. Built on the host with swiftc and
// copied into the guest by p19-rig.sh tray-items; it only ever runs in the VM (19-15 F1).
let a = CommandLine.arguments
guard a.count == 2 else { print("usage: axtray bundleID"); exit(2) }
guard let app = NSRunningApplication.runningApplications(withBundleIdentifier: a[1]).first else {
  print("app not running"); exit(1)
}
let ax = AXUIElementCreateApplication(app.processIdentifier)
func attr(_ e: AXUIElement, _ n: String) -> AnyObject? {
  var v: AnyObject?
  AXUIElementCopyAttributeValue(e, n as CFString, &v)
  return v
}
func kids(_ e: AXUIElement) -> [AXUIElement] { (attr(e, kAXChildrenAttribute) as? [AXUIElement]) ?? [] }
func title(_ e: AXUIElement) -> String { (attr(e, kAXTitleAttribute) as? String) ?? "" }
guard let extras = attr(ax, "AXExtrasMenuBar") else { print("no status items"); exit(1) }
let items = kids(extras as! AXUIElement)
print("status items:", items.count)
for item in items {
  AXUIElementPerformAction(item, kAXPressAction as CFString)
  usleep(800_000)
  guard let menu = kids(item).first else { print("status item has no menu"); continue }
  for mi in kids(menu) {
    let t = title(mi)
    if t.isEmpty { continue }
    let enabled = (attr(mi, kAXEnabledAttribute) as? Bool) ?? true
    print("tray-item: \(t) enabled=\(enabled ? "yes" : "no")")
  }
  AXUIElementPerformAction(menu, "AXCancel" as CFString)
  usleep(300_000)
}
