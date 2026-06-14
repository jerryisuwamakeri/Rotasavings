export default function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="container inner">
        <div className="brand" style={{ fontSize: 15 }}>Rotasavings</div>
        <div>Truth on-chain. Stability through intelligence.</div>
        <div>&copy; {new Date().getFullYear()} Rotasavings</div>
      </div>
    </footer>
  );
}
