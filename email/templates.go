package email

const verificationTmpl = `<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:32px;background:#0f172a;color:#e2e8f0">
  <div style="background:#1e293b;border:1px solid #334155;border-radius:12px;padding:32px">
    <h2 style="color:#818cf8;margin-top:0">Verify your email</h2>
    <p>Hi <strong>{{.Name}}</strong>, thanks for signing up to Docker Wrapper.</p>
    <p>Click the button below to verify your email address.</p>
    <a href="{{.URL}}"
       style="display:inline-block;background:#6366f1;color:#fff;padding:12px 28px;border-radius:8px;text-decoration:none;font-weight:600;margin:16px 0">
      Verify Email
    </a>
    <p style="color:#64748b;font-size:13px;margin-top:24px">
      This link expires in <strong>24 hours</strong>.<br>
      If you didn't create an account, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`

const passwordResetTmpl = `<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:32px;background:#0f172a;color:#e2e8f0">
  <div style="background:#1e293b;border:1px solid #334155;border-radius:12px;padding:32px">
    <h2 style="color:#818cf8;margin-top:0">Reset your password</h2>
    <p>Hi <strong>{{.Name}}</strong>, we received a request to reset your password.</p>
    <a href="{{.URL}}"
       style="display:inline-block;background:#6366f1;color:#fff;padding:12px 28px;border-radius:8px;text-decoration:none;font-weight:600;margin:16px 0">
      Reset Password
    </a>
    <p style="color:#64748b;font-size:13px;margin-top:24px">
      This link expires in <strong>1 hour</strong>.<br>
      If you didn't request a password reset, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`
