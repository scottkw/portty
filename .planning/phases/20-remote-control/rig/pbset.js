// pbset.js: osascript -l JavaScript pbset.js BASE64_TEXT 0|1  (1 marks the item concealed).
// Shipped to the guest by p20-rig.sh pb-set and run only there. In JXA a method without arguments is a
// property (pb.clearContents runs it), and $() is an empty dictionary, not nil: the owner is null.
ObjC.import('AppKit')
function run(argv) {
  const data = $.NSData.alloc.initWithBase64EncodedStringOptions(argv[0], 0)
  const text = $.NSString.alloc.initWithDataEncoding(data, $.NSUTF8StringEncoding)
  const conceal = argv[1] === '1'
  const pb = $.NSPasteboard.generalPasteboard
  pb.clearContents
  const types = conceal ? ['public.utf8-plain-text', 'org.nspasteboard.ConcealedType'] : ['public.utf8-plain-text']
  pb.declareTypesOwner($(types), null)
  pb.setStringForType(text, 'public.utf8-plain-text')
  if (conceal) pb.setStringForType($('1'), 'org.nspasteboard.ConcealedType')
  return 'types: ' + ObjC.deepUnwrap(pb.types).join(',')
}
