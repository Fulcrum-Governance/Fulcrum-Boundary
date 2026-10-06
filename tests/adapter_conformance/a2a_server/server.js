import express from 'express';

const app = express();
app.use(express.json());

app.post('/a2a/rpc_manual', (req, res) => {
    const { method, params, id } = req.body;

    if (method === 'message/stream') {
        res.setHeader('Content-Type', 'text/event-stream');
        res.flushHeaders();
        res.write('data: ' + JSON.stringify({jsonrpc: "2.0", result: "started"}) + '\n\n');
        res.end();
        return;
    }

    if (method === 'tasks/send' || method === 'message/send') {
        const action = params?.metadata?.action;
        const text = params?.message?.parts?.[0]?.data?.text;

        if (action === 'summarize') {
            return res.json({
                jsonrpc: "2.0",
                id,
                result: {
                   status: 'success',
                   message: {
                       parts: [{ kind: 'data', data: { text: `Summarized: ${text}` } }]
                   }
                }
            });
        }

        if (action === 'large_payload') {
            return res.json({
                jsonrpc: "2.0",
                id,
                result: {
                   status: 'success',
                   message: {
                       parts: [{ kind: 'data', data: { text: 'x'.repeat(10000) } }]
                   }
                }
            });
        }

        return res.json({
            jsonrpc: "2.0",
            id,
            error: { code: -32601, message: `Unknown action: ${action}` }
        });
    }

    return res.json({
        jsonrpc: "2.0",
        id,
        error: { code: -32601, message: `Method not found: ${method}` }
    });
});

const server = app.listen(0, '127.0.0.1', () => {
    console.log(`PORT:${server.address().port}`);
});
