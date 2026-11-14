"""Screenshot ganglion pages in both themes and widths; report console errors and bytes."""
import sys, json, asyncio
from playwright.async_api import async_playwright

BASE = "http://localhost:7171"
OUT = sys.argv[1]
PAGES = sys.argv[2].split(",")
WIDTHS = [("desk", 1360, 900), ("phone", 390, 844)]


async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        for theme in ("light", "dark"):
            for wname, w, h in WIDTHS:
                ctx = await b.new_context(viewport={"width": w, "height": h}, color_scheme=theme, device_scale_factor=1)
                page = await ctx.new_page()
                errs, total = [], {"bytes": 0}
                page.on("console", lambda m: errs.append(m.text) if m.type in ("error", "warning") else None)
                page.on("pageerror", lambda e: errs.append(str(e)))
                async def resp(r):
                    try:
                        sz = await r.request.sizes()
                        total["bytes"] += sz["responseBodySize"] + sz["responseHeadersSize"]
                    except Exception:
                        pass
                page.on("requestfinished", lambda r: asyncio.ensure_future(resp(r)))
                for name in PAGES:
                    path, _, wait = name.partition("@")
                    total["bytes"] = 0
                    await page.goto(BASE + path, wait_until="networkidle")
                    if wait:
                        await page.wait_for_timeout(int(wait))
                    await page.wait_for_timeout(300)
                    fn = f"{OUT}/{path.strip('/').replace('/', '_').replace('?', '_').replace('=', '-').replace('&', '_') or 'home'}-{theme}-{wname}.png"
                    await page.screenshot(path=fn, full_page=True)
                    over = await page.evaluate("""() => { if (document.documentElement.scrollWidth <= innerWidth) return false;
                      return [...document.querySelectorAll('body *')].filter(e => e.getBoundingClientRect().right > innerWidth + 1)
                        .slice(0, 6).map(e => e.tagName + '.' + e.className + ' ' + Math.round(e.getBoundingClientRect().right)); }""")
                    print(json.dumps({"page": name, "theme": theme, "w": wname, "kb": round(total["bytes"] / 1024, 1), "hscroll": over, "errors": errs[:5]}))
                    errs.clear()
                await ctx.close()
        await b.close()

asyncio.run(main())
