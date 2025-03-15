import Link from "next/link"
import { redirect } from "next/navigation"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

import { signUpUser } from "./action"

export default function SignUpPage() {
  return (
    <div className="flex h-screen w-full items-center justify-center">
      <div className="mx-auto flex w-full flex-col justify-center space-y-6 sm:w-[350px]">
        <div className="flex flex-col space-y-2 text-center">
          <h1 className="text-2xl font-semibold tracking-tight">
            Create an account
          </h1>
          <p className="text-muted-foreground text-sm">
            Enter your details to sign up
          </p>
        </div>
        <form
          action={async (formData) => {
            "use server"
            const email = formData.get("email") as string
            const username = formData.get("username") as string
            const password = formData.get("password") as string
            const result = await signUpUser({ email, username, password })

            if (result?.error) {
              // Handle error here
              console.error(result.error)
              return
            }

            redirect("/login")
          }}
          className="w-[350px]"
        >
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                name="email"
                type="email"
                placeholder="Enter your email"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                name="username"
                type="text"
                placeholder="Choose a username"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                name="password"
                type="password"
                placeholder="Create a password"
                required
              />
            </div>
            <Button type="submit">Sign Up</Button>

            <div className="text-center text-sm">
              Already have an account?{" "}
              <Link
                href="/login"
                className="text-primary underline underline-offset-4"
              >
                Login
              </Link>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
