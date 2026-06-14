import Link from "next/link";
import SiteHeader from "./components/SiteHeader";
import SiteFooter from "./components/SiteFooter";

export default function LandingPage() {
  return (
    <>
      <SiteHeader />

      {/* Hero */}
      <section className="hero">
        <div className="container grid">
          <div>
            <span className="kicker">Ajo &middot; Esusu &middot; Stokvel</span>
            <h1>
              Save together.<br />
              <em>Trust</em> the math.
            </h1>
            <p className="lead">
              Rotasavings turns the savings circles people already run on trust into circles
              the rules keep for you - every contribution, payout and default settled by smart
              contract, never by argument.
            </p>
            <div className="cta-row">
              <Link href="/login" className="btn btn-primary btn-lg">Start a group</Link>
              <a href="#how" className="btn btn-text">See how it works</a>
            </div>
            <div className="trust">
              <span><b>100%</b> on-chain history</span>
              <span><b>Zero</b> fake payments</span>
              <span><b>Portable</b> reputation</span>
            </div>
          </div>

          {/* Hand-built product mock instead of an abstract graphic */}
          <div className="mock" aria-hidden="true">
            <div className="bar"><span /><span /><span /></div>
            <div className="body">
              <div className="top">
                <h4>Lagos Traders Circle</h4>
                <div className="amt"><b>&#8358;50,000</b> / cycle</div>
              </div>
              <ul>
                <li>
                  <span className="who"><span className="ava">A</span> Amara</span>
                  <span className="done">paid &middot; received</span>
                </li>
                <li>
                  <span className="who"><span className="ava">K</span> Kunle</span>
                  <span className="turn">cycle 2 &middot; paying out</span>
                </li>
                <li>
                  <span className="who"><span className="ava">N</span> Ngozi</span>
                  <span className="next">up next</span>
                </li>
                <li>
                  <span className="who"><span className="ava">T</span> Tunde</span>
                  <span className="next">cycle 4</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="block" id="how">
        <div className="container">
          <div className="section-head">
            <span className="kicker">How it works</span>
            <h2>A savings circle, with the <em>trust built in.</em></h2>
            <p>Everyone contributes each cycle; one member takes the pooled payout. Rotasavings makes the whole rotation verifiable and self-enforcing.</p>
          </div>
          <div className="steps">
            <Step n="01" title="Join a verified group" body="Complete KYC once. Your identity is anchored on-chain as a private commitment - unique across every group, revealing nothing about you." />
            <Step n="02" title="Contribute each cycle" body="Each contribution is bound to a cryptographic commitment and collected through escrow. Nothing fake, nothing edited after the fact." />
            <Step n="03" title="Receive your payout" body="When the rotation reaches you, the pooled funds are disbursed and recorded as truth - on time, in order, no disputes." />
            <Step n="04" title="Carry your reputation" body="Every contribution, payout and missed payment becomes an auditable event you take with you to the next group." />
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="block">
        <div className="container">
          <div className="section-head">
            <span className="kicker">The platform</span>
            <h2>Truth you can&apos;t argue with. <em>Stability</em> you can plan around.</h2>
            <p>A deterministic chain that enforces the rules, and an intelligence engine that decides who fits which group and flags trouble early.</p>
          </div>
          <div className="feat-grid">
            <Feat label="Truth layer" title="Smart-contract enforcement" body="Group rules, contributions, payouts and defaults live on-chain and become immutable once a group is active." />
            <Feat label="Prediction" title="Default-risk scoring" body="Every prospective member gets an explainable risk score from their history and behavior before a group forms." />
            <Feat label="Matching" title="Group optimizer" body="Members are composed into balanced groups that minimize correlated default risk and maximize survival." />
            <Feat label="Monitoring" title="Early-warning signals" body="Active cycles are watched for drift, so organizers get a heads-up - never a nasty surprise on payout day." />
            <Feat label="Money" title="Escrow & local rails" body="Contributions are collected and paid out through escrow and mobile-money or bank rails. The chain instructs; money executes." />
            <Feat label="Identity" title="Portable reputation" body="Reputation is a deterministic fold of on-chain events - auditable, not a black box - and it follows you everywhere." />
          </div>
        </div>
      </section>

      {/* Two layers */}
      <section className="block">
        <div className="container">
          <div className="section-head">
            <span className="kicker">Two layers, one promise</span>
            <h2>The chain keeps the record. <em>Intelligence</em> keeps the peace.</h2>
          </div>
          <div className="split">
            <div className="layer truth">
              <div className="tag">System of truth</div>
              <h3>Deterministic. Cryptographic.</h3>
              <p>It cannot be argued with, because there is nothing to argue about.</p>
              <ul>
                <li>Identity commitments anchor KYC without exposing anyone.</li>
                <li>Contribution commitments make every payment verifiable.</li>
                <li>Defaults are state failures, not opinions.</li>
                <li>Reputation is an event ledger, portable across all groups.</li>
              </ul>
            </div>
            <div className="layer intel">
              <div className="tag">Intelligence layer</div>
              <h3>Advisory. Explainable.</h3>
              <p>It assists every decision and enforces none of them.</p>
              <ul>
                <li>Predicts default probability before a group forms.</li>
                <li>Balances high- and low-risk members across groups.</li>
                <li>Flags deviations mid-cycle for early intervention.</li>
                <li>Forecasts liquidity stress across the whole system.</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* Security */}
      <section className="block" id="security">
        <div className="container">
          <div className="section-head">
            <span className="kicker">Security &amp; trust</span>
            <h2>Privacy by design, <em>truth</em> by default.</h2>
            <p>KYC happens off-chain; the chain stores only a hash. The database caches on-chain state and guards your application data - it is never the arbiter of what happened.</p>
          </div>
          <div className="feat-grid">
            <Feat label="Privacy" title="Hashes, not identities" body="On-chain we store H(identity) only - preventing duplicate accounts while revealing nothing personal." />
            <Feat label="Audit" title="Replayable from genesis" body="Every reputation-affecting event is recorded and can be rebuilt from block height." />
            <Feat label="Custody" title="Money never decides" body="The payment layer only carries out instructions from the contracts. It can never rewrite the truth." />
            <Feat label="Control" title="Rules are immutable" body="Once a group is active, its contribution amount, cycle length and payout order cannot be quietly changed." />
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="cta">
        <div className="container">
          <div className="cta-inner">
            <h2>Start a circle that can&apos;t be cheated.</h2>
            <p>Contribute with confidence, get paid in turn, and build reputation that travels with you.</p>
            <div className="cta-row">
              <Link href="/login" className="btn btn-primary btn-lg">Start a group</Link>
              <Link href="/login" className="btn btn-ghost btn-lg">Operator sign in</Link>
            </div>
          </div>
        </div>
      </section>

      <SiteFooter />
    </>
  );
}

function Step({ n, title, body }: { n: string; title: string; body: string }) {
  return (
    <div className="step">
      <div className="n">{n}</div>
      <div>
        <h4>{title}</h4>
        <p>{body}</p>
      </div>
    </div>
  );
}

function Feat({ label, title, body }: { label: string; title: string; body: string }) {
  return (
    <div className="feat">
      <div className="lbl">{label}</div>
      <h3>{title}</h3>
      <p>{body}</p>
    </div>
  );
}
