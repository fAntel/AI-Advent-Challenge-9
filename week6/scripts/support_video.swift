// Render readable video cards with native macOS graphics; no Python packages.
import AppKit
import Foundation

let args = CommandLine.arguments
guard args.count == 3 else { fatalError("usage: support_video scenes.json output-directory") }
let scenes = try JSONSerialization.jsonObject(with: Data(contentsOf: URL(fileURLWithPath: args[1]))) as! [[String: Any]]
let out = URL(fileURLWithPath: args[2], isDirectory: true)
try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
let width = 1920, height = 1080
func color(_ hex: UInt32) -> NSColor {
    NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255, green: CGFloat((hex >> 8) & 255) / 255, blue: CGFloat(hex & 255) / 255, alpha: 1)
}
let ink = color(0xF0F5FF), muted = color(0xAAB8CE), mint = color(0x77E3BE)
func rect(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat) -> NSRect {
    NSRect(x: x, y: CGFloat(height) - y - h, width: w, height: h)
}
func box(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat, _ fill: NSColor, radius: CGFloat = 18) {
    fill.setFill(); NSBezierPath(roundedRect: rect(x,y,w,h), xRadius: radius, yRadius: radius).fill()
}
func text(_ value: String, _ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat, size: CGFloat = 28, weight: NSFont.Weight = .regular, tint: NSColor = ink, mono: Bool = false) {
    let p = NSMutableParagraphStyle(); p.lineSpacing = 7; p.lineBreakMode = .byWordWrapping
    var fontSize = size
    var content: NSAttributedString
    while true {
        let f = mono ? NSFont.monospacedSystemFont(ofSize: fontSize, weight: weight) : NSFont.systemFont(ofSize: fontSize, weight: weight)
        let attrs: [NSAttributedString.Key: Any] = [.font:f, .foregroundColor:tint, .paragraphStyle:p]
        content = NSAttributedString(string:value, attributes:attrs)
        let measured = content.boundingRect(with:NSSize(width:w,height:10000), options:[.usesLineFragmentOrigin,.usesFontLeading]).height
        if measured <= h || fontSize <= 20 { break }
        fontSize -= 1
    }
    content.draw(with:rect(x,y,w,h), options:[.usesLineFragmentOrigin,.usesFontLeading])
}
for (index, scene) in scenes.enumerated() {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes:nil, pixelsWide:width, pixelsHigh:height, bitsPerSample:8, samplesPerPixel:4, hasAlpha:true, isPlanar:false, colorSpaceName:.deviceRGB, bytesPerRow:0, bitsPerPixel:0)!
    let context = NSGraphicsContext(bitmapImageRep:bitmap)!
    NSGraphicsContext.saveGraphicsState(); NSGraphicsContext.current = context
    box(0,0,1920,1080,color(0x0C1423),radius:0)
    box(0,0,12,1080,mint,radius:0)
    text("WEEK 6 / TASK 2",72,42,500,35,size:21,weight:.semibold,tint:mint)
    box(1300,35,545,48,color(0x173B35),radius:24)
    text("REAL AI RUN  ·  SIMULATED TRANSPORT",1320,46,510,32,size:19,weight:.semibold,tint:mint)
    text(scene["title"] as! String,72,100,1770,75,size:44,weight:.bold)
    text(scene["subtitle"] as! String,74,171,1750,45,size:22,tint:muted)
    box(72,238,1120,732,color(0x152237))
    text(scene["questionLabel"] as! String,102,265,1020,40,size:18,weight:.bold,tint:mint)
    text(scene["question"] as! String,102,309,1040,145,size:27,weight:.medium)
    box(102,458,1060,2,color(0x2B3B53),radius:0)
    text(scene["bodyLabel"] as! String,102,483,1020,40,size:18,weight:.bold,tint:mint)
    text(scene["body"] as! String,102,526,1040,416,size:27)
    box(1224,238,624,732,color(0x111E30))
    text(scene["sideLabel"] as! String,1255,266,560,40,size:18,weight:.bold,tint:mint)
    text(scene["side"] as! String,1255,312,560,520,size:24,mono:scene["mono"] as? Bool ?? false)
    box(1254,858,564,78,color(0x223C3C))
    text(scene["badge"] as! String,1276,878,520,50,size:23,weight:.semibold,tint:mint)
    text("ANNOTATED REPLAY  ·  Actual questions, answers and tool records from the verified capture",74,1003,1650,38,size:19,tint:muted)
    text(String(format:"%02d / %02d",index+1,scenes.count),1736,1003,125,35,size:19,weight:.semibold,tint:mint)
    NSGraphicsContext.restoreGraphicsState()
    let png = bitmap.representation(using:.png, properties:[:])!
    try png.write(to:out.appendingPathComponent(String(format:"scene-%02d.png",index)))
}
