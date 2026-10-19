import "./globals.css";
export const metadata = {
  title: "Resource Management",
  description: "Engineering capacity, delivery and cost planning",
};
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
