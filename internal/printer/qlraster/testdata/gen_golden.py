#!/usr/bin/env python3
"""qlraster の golden データを brother_ql で生成する。

brother_ql (https://github.com/pklaus/brother_ql) は Brother QL シリーズへ
ラスタを直接送る実績のある実装で、QL-820NWB も対象に含む。hub の Go 実装
(qlraster.Encode) は、同じ画像から brother_ql とバイト単位で同一の出力を
作ることをテストで保証する。

再生成:
  python3 -m venv /tmp/bql && /tmp/bql/bin/pip install brother_ql==0.9.4 Pillow
  /tmp/bql/bin/python gen_golden.py

brother_ql へ渡す条件（qlraster の既定と一致させている）:
  model=QL-820NWB label=62（62mm 連続テープ） rotate=0 threshold=50
  dither=False compress=False red=False dpi_600=False hq=True
732 dot 幅の画像は、hub と同じく中央 696 dot を切り出してから渡す
（brother_ql は幅が違うと縮小してしまうため）。
"""
import os
from PIL import Image, ImageDraw
from brother_ql.conversion import convert
from brother_ql.raster import BrotherQLRaster

HERE = os.path.dirname(os.path.abspath(__file__))
PRINTABLE = 696


def make_synthetic():
    # 端の配置と左右反転を 1 dot 単位で確かめる
    im = Image.new('L', (PRINTABLE, 160), 255)
    d = ImageDraw.Draw(im)
    d.line([(0, 0), (0, 159)], fill=0)                  # 左端 1 dot
    d.line([(PRINTABLE - 1, 0), (PRINTABLE - 1, 159)], fill=0)  # 右端 1 dot
    d.line([(0, 0), (PRINTABLE - 1, 0)], fill=0)        # 先頭行
    d.rectangle([(10, 20), (60, 70)], fill=0)           # 左上の塊（左右の取り違え検出用）
    im.save(os.path.join(HERE, 'edges_696.png'))

    # しきい値の境目: 列 x の濃度 = x * 255 // 695
    im = Image.new('L', (PRINTABLE, 150), 255)
    px = im.load()
    for x in range(PRINTABLE):
        v = x * 255 // (PRINTABLE - 1)
        for y in range(150):
            px[x, y] = v
    im.save(os.path.join(HERE, 'gradient_696.png'))

    # 真っ白: 全行インクなし
    Image.new('L', (PRINTABLE, 150), 255).save(os.path.join(HERE, 'blank_696.png'))


def encode(path, cut=True, pages=1, model='QL-820NWB'):
    im = Image.open(path)
    if im.size[0] == 732:
        left = (732 - PRINTABLE) // 2
        im = im.crop((left, 0, left + PRINTABLE, im.size[1]))
    qlr = BrotherQLRaster(model)
    qlr.exception_on_warning = True
    return convert(qlr=qlr, images=[im] * pages, label='62', rotate=0, threshold=50.0,
                   dither=False, compress=False, red=False, dpi_600=False,
                   hq=True, cut=cut)


if __name__ == '__main__':
    make_synthetic()
    cases = [
        ('traceable_732.png', True, 'traceable_732.bin'),
        ('edges_696.png', True, 'edges_696.bin'),
        ('gradient_696.png', True, 'gradient_696.bin'),
        ('blank_696.png', True, 'blank_696.bin'),
        ('edges_696.png', False, 'edges_696_nocut.bin'),
    ]
    for src, cut, dst in cases:
        data = encode(os.path.join(HERE, src), cut=cut)
        with open(os.path.join(HERE, dst), 'wb') as f:
            f.write(data)
        print(f'{dst}: {len(data)} bytes')

    # 部数指定: brother_ql は初期化を1回だけ行い、ページごとに情報・ラスタ・印字を繰り返す
    data = encode(os.path.join(HERE, 'edges_696.png'), pages=2)
    with open(os.path.join(HERE, 'edges_696_x2.bin'), 'wb') as f:
        f.write(data)
    print(f'edges_696_x2.bin: {len(data)} bytes')

    # hub が対応する QL-800 も、非圧縮なら QL-820NWB と同じバイト列になることを確かめる
    assert encode(os.path.join(HERE, 'edges_696.png'), model='QL-800') == \
        encode(os.path.join(HERE, 'edges_696.png'), model='QL-820NWB')
    print('QL-800 == QL-820NWB: ok')
