import React, { useState, useEffect } from 'react'

export default function Dashboard() {
  const [data, setData] = useState(null)

  useEffect(() => {
    fetch('/metrics')
      .then(r => r.json())
      .then(setData)
  }, [])

  return (
    <div>
      <h1>Deployment Dashboard</h1>
      {data && <pre>{JSON.stringify(data, null, 2)}</pre>}
    </div>
  )
}
