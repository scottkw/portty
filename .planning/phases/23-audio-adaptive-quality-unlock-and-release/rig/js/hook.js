// Keep the viewer's RTCPeerConnection reachable: the page's debug poll calls getStats() once a second,
// so wrapping the prototype method captures the instance in window.__pc within about a second.
const o = RTCPeerConnection.prototype.getStats
if (!window.__hooked) {
  window.__hooked = true
  RTCPeerConnection.prototype.getStats = function () {
    window.__pc = this
    return o.apply(this, arguments)
  }
}
await new Promise((r) => setTimeout(r, 2500))
return window.__pc ? 'hooked' : 'no-pc'
