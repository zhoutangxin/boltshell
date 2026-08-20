from PIL import Image

src = r"E:\resource\person\BoltShell\docs\logo\boltshell-logo-icon-v1.png"
ico = r"E:\resource\person\BoltShell\build\windows\icon.ico"
img = Image.open(src).convert("RGBA")
sizes = [(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)]
img.save(ico, format="ICO", sizes=sizes)
print("icon.ico saved:", ico)
