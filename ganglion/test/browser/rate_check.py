"""Check rating on the Samples page against a test server. Usage: rate_check.py BASE OUTDIR
Run it against a separate instance started with -user claude-test, so no test label
is ever recorded as the real labeller's."""
import asyncio
import sys

from playwright.async_api import async_playwright

BASE, OUT = sys.argv[1], sys.argv[2]


async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        page = await b.new_page(viewport={"width": 1360, "height": 900}, color_scheme="light")
        errs = []
        page.on("pageerror", lambda e: errs.append(str(e)))
        page.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
        await page.goto(BASE + "/samples?set=d:prose-v2/test&speaker=claude&seed=42&n=5", wait_until="networkidle")
        await page.keyboard.press("1")   # good on #1, moves to #2
        await page.wait_for_timeout(300)
        await page.keyboard.press("3")   # not prose on #2
        await page.wait_for_timeout(300)
        await page.click(".sample:nth-of-type(4) [data-rate=bad]")
        await page.wait_for_timeout(500)
        counts = [await page.text_content(f"#count-{k}") for k in ("good", "bad", "not-prose")]
        print("counts after rating:", counts)
        await page.screenshot(path=f"{OUT}/samples-rated.png", full_page=True)
        await page.reload(wait_until="networkidle")
        pressed = await page.locator(".sample [aria-pressed=true]").all_text_contents()
        print("after reload, pressed:", pressed)
        await page.keyboard.press("1")   # pressing the same rating again clears #1
        await page.wait_for_timeout(500)
        print("after clearing #1:", [await page.text_content(f"#count-{k}") for k in ("good", "bad", "not-prose")])
        print("errors:", errs)
        await b.close()

asyncio.run(main())
