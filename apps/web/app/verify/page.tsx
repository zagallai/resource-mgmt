"use client";
import { useState } from "react";
export default function Verify() {
  const [message, setMessage] = useState(
    "Confirm sign-in to Resource Compass.",
  );
  const [busy, setBusy] = useState(false);
  async function verify() {
    setBusy(true);
    try {
      const token = new URLSearchParams(location.hash.slice(1)).get("token");
      const r = await fetch("/auth/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
      if (!r.ok)
        throw new Error(
          "This link has expired or already been used. Request a new one.",
        );
      history.replaceState(null, "", "/verify");
      location.href = "/";
    } catch (e) {
      setMessage((e as Error).message);
      setBusy(false);
    }
  }
  return (
    <main className="login">
      <div className="brand">◈ Resource Compass</div>
      <h1>Welcome aboard</h1>
      <p role="status">{message}</p>
      <button disabled={busy} onClick={verify}>
        Confirm sign-in
      </button>
      <p>
        <a href="/">Return to sign-in</a>
      </p>
    </main>
  );
}
