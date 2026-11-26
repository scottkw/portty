// readterm.js: run inside the terminal share page (Safari WebDriver execute/sync, the body is a
// function body). Finds the xterm instance through the React fiber tree and reports the buffer:
// its length, the last TICK line and a contiguity check over every "TICK n epoch" line the
// ticking test shells printed (first, last, count, breaks = non-consecutive steps). 19-15 F5/F6.
var el=document.querySelector('.terminal-wrapper')||document.querySelector('.terminal');
if(!el) return JSON.stringify({err:'no-terminal-element'});
var k=Object.keys(el).find(function(x){return x.indexOf('__reactFiber')===0});
var top=el[k]; while(top.return) top=top.return;
var seen=new Set(), found=null;
function isTerm(v){ try{ return v&&typeof v==='object'&&v.buffer&&v.buffer.active&&typeof v.buffer.active.getLine==='function'; }catch(e){ return false; } }
function scan(v,d){ if(found||!v||typeof v!=='object'||d>3||seen.has(v)) return; if(v instanceof Node||v instanceof Window) return; seen.add(v); if(isTerm(v)){ found=v; return; } var ks; try{ ks=Object.keys(v); }catch(e){ return; } for(var i=0;i<ks.length&&i<40;i++){ var c; try{ c=v[ks[i]]; }catch(e){ continue; } scan(c,d+1); } }
var stack=[top];
while(stack.length&&!found){ var x=stack.pop(); var s=x.memoizedState; var m=0; while(s&&m<50&&!found){ scan(s.memoizedState,0); s=s.next; m++; } if(x.sibling) stack.push(x.sibling); if(x.child) stack.push(x.child); }
if(!found) return JSON.stringify({err:'no-term'});
var b=found.buffer.active, first=null, last=null, lastEpoch=null, count=0, breaks=0, prev=null;
for(var i=0;i<b.length;i++){
  var l=b.getLine(i); if(!l) continue;
  var mm=/TICK (\d+) (\d+)/.exec(l.translateToString(true)); if(!mm) continue;
  var n=parseInt(mm[1],10); count++; if(first===null) first=n;
  if(prev!==null&&n!==prev+1) breaks++;
  prev=n; last=n; lastEpoch=parseInt(mm[2],10);
}
return JSON.stringify({len:b.length,first:first,last:last,lastEpoch:lastEpoch,count:count,breaks:breaks});
