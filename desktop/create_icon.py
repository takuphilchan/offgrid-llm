#!/usr/bin/env python3
from PIL import Image, ImageDraw
import sys

# Create a 512x512 image with gradient
size = 512
img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
draw = ImageDraw.Draw(img)

# Draw gradient circle background
for i in range(240):
    r = int(249 - (249 - 6) * i / 240)
    g = int(115 + (182 - 115) * i / 240)
    b = int(22 + (212 - 22) * i / 240)
    draw.ellipse([256-240+i, 256-240+i, 256+240-i, 256+240-i], 
                 fill=(r, g, b, 255))

# Draw lightning bolt
lightning = [
    (280, 100), (180, 280), (240, 280), 
    (220, 420), (340, 220), (280, 220)
]
draw.polygon(lightning, fill=(255, 255, 255, 255))

# Save
img.save('icon.png')
print('✓ Icon created: icon.png')
