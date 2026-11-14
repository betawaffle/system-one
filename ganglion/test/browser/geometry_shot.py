"""Screenshot Similarity in each space with the geometry panel. Usage: geometry_shot.py OUTDIR"""
import asyncio
import sys

from playwright.async_api import async_playwright

OUT = sys.argv[1]
TEXTS = "ls -la\nls -al\nfind . -maxdepth 1\npytest -x -q\ncat README.md\nThe weather is lovely today."


async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        for theme in ("light", "dark"):
            page = await b.new_page(viewport={"width": 1360, "height": 900}, color_scheme=theme)
            errs = []
            page.on("pageerror", lambda e: errs.append(str(e)))
            page.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
            await page.goto("http://localhost:7171/ettin", wait_until="networkidle")
            await page.click("#model button[data-model=both]")
            await page.click("#modes button[data-mode=similarity]")
            await page.fill("#text", TEXTS)
            await page.dispatch_event("#text", "input")
            for sp in ("raw", "centred", "whitened"):
                await page.click(f"#space button[data-space={sp}]")
                for _ in range(60):  # first time builds corpus stats
                    await page.wait_for_timeout(1000)
                    st = await page.text_content("#status-text")
                    if "round trip" in st and await page.locator("#geometry .multiples").count() >= 2:
                        break
                cells = await page.locator(".pane .simtable tr:nth-child(2) td").all_text_contents()
                print(theme, sp, "row 1 (ls -la vs others):", cells, "|", st[:50])
                await page.screenshot(path=f"{OUT}/ettin-sim-{sp}-{theme}.png", full_page=True)
            print(theme, "errors:", errs)
            await page.close()
        await b.close()

asyncio.run(main())
