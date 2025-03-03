import { NextResponse, type NextRequest } from "next/server"

export const verifyToken = async (token: string) => {
  try {
    const response = await fetch(`${process.env.API_URL}/api/jobs`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })

    if (response.status === 401) {
      return false
    }
    return true
  } catch (error) {
    return false
  }
}

export async function middleware(request: NextRequest) {
  // Allow access to login page and API routes
  if (request.nextUrl.pathname.startsWith("/api/")) {
    return NextResponse.next()
  }

  if (request.nextUrl.pathname === "/login") {
    const token = request.cookies.get("token")
    if (token) {
      const isVerified = await verifyToken(token.value)
      if (isVerified) {
        return NextResponse.redirect(new URL("/", request.url))
      }
    }
    return NextResponse.next()
  }

  const token = request.cookies.get("token")

  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url))
  }

  const isVerified = await verifyToken(token.value)
  console.log("isVerified", isVerified)
  if (!isVerified) {
    return NextResponse.redirect(new URL("/login", request.url))
  }

  return NextResponse.next()
}

// Configure which routes to run middleware on
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * 1. /api routes
     * 2. /_next (Next.js internals)
     * 3. /_static (inside /public)
     * 4. /_vercel (Vercel internals)
     * 5. /favicon.ico, /sitemap.xml (static files)
     */
    "/((?!api|_next|_static|_vercel|favicon.ico|sitemap.xml).*)",
  ],
}
