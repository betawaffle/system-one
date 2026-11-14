import asyncio, sys, json
from playwright.async_api import async_playwright

OUT = sys.argv[1]
TB = 'Traceback (most recent call last):\n  File "/testbed/src/parser.py", line 88, in parse\n    return self.table[key]\nKeyError: \'encoding\''


async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        for theme in ("light", "dark"):
            ctx = await b.new_context(viewport={"width": 1360, "height": 900}, color_scheme=theme)
            page = await ctx.new_page()
            errs = []
            page.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
            page.on("pageerror", lambda e: errs.append(str(e)))
            await page.goto("http://localhost:7171/ettin", wait_until="networkidle")
            await page.fill("#text", TB)
            await page.dispatch_event("#text", "input")
            await page.wait_for_timeout(1500)
            await page.click("#model button[data-model=both]")
            await page.wait_for_timeout(1500)
            await page.screenshot(path=f"{OUT}/ettin-surprise-{theme}.png", full_page=True)
            print(theme, "surprise status:", await page.text_content("#status-text"))
            await page.click("text=Fill: Git")
            await page.wait_for_timeout(1500)
            await page.screenshot(path=f"{OUT}/ettin-fill-{theme}.png", full_page=True)
            await page.click("text=Similarity Set")
            await page.wait_for_timeout(1500)
            await page.screenshot(path=f"{OUT}/ettin-sim-{theme}.png", full_page=True)
            await page.click("#modes button[data-mode=tokens]")
            await page.fill("#text", "cd /testbed && pytest tests/test_api.py::test_get_user -x -q 2>&1 | tail -20")
            await page.dispatch_event("#text", "input")
            await page.wait_for_timeout(1500)
            await page.screenshot(path=f"{OUT}/ettin-tokens-{theme}.png", full_page=True)
            print(theme, "errors:", errs, "status:", await page.text_content("#status-text"))
            await ctx.close()
        await b.close()

asyncio.run(main())
