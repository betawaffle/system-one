"""Screenshot the Ettin Invert tab for a few commands. Usage: invert_shot.py OUTDIR"""
import asyncio
import sys

from playwright.async_api import async_playwright

OUT = sys.argv[1]
CMDS = ["git log --oneline -5", "cd /testbed && python -m pytest tests/test_parser.py -x -q 2>&1 | tail -20",
        "grep -rn \"def parse_config\" src/ | head -20"]


async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        for theme in ("light", "dark"):
            page = await b.new_page(viewport={"width": 1360, "height": 900}, color_scheme=theme)
            errs = []
            page.on("pageerror", lambda e: errs.append(str(e)))
            await page.goto("http://localhost:7171/ettin", wait_until="networkidle")
            await page.click("#modes button[data-mode=invert]")
            for i, c in enumerate(CMDS):
                await page.fill("#text", c)
                await page.dispatch_event("#text", "input")
                await page.wait_for_timeout(6000 if i == 0 else 2500)
                hyp = await page.locator(".diff").all_text_contents()
                print(theme, repr(c), "->", hyp[:1], (await page.text_content("#status-text"))[:60])
            await page.screenshot(path=f"{OUT}/ettin-invert-{theme}.png", full_page=True)
            print(theme, "errors:", errs)
            await page.close()
        await b.close()

asyncio.run(main())
