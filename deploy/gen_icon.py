from pathlib import Path
from PIL import Image

root = Path(__file__).resolve().parents[1]
src = root / "docs/product/logo/boltshell-logo-icon-v1.png"
ico = root / "server/build/windows/icon.ico"
img = Image.open(src).convert("RGBA")
sizes = [(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)]
img.save(ico, format="ICO", sizes=sizes)
print("icon.ico saved:", ico)
