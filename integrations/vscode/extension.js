const cp = require('child_process');
const vscode = require('vscode');

function cliPath() {
    return process.env.PROOFBOARD_CLI_PATH || 'proofboard';
}

function announceWorkspace(workspacePath, editorName) {
    return new Promise((resolve) => {
        const args = ['detect', '--workspace', workspacePath, '--editor', editorName, '--json'];
        const child = cp.spawn(cliPath(), args, {
            env: process.env,
            stdio: ['ignore', 'pipe', 'pipe'],
        });
        let stdout = '';
        let stderr = '';
        child.stdout.on('data', (chunk) => {
            stdout += chunk.toString();
        });
        child.stderr.on('data', (chunk) => {
            stderr += chunk.toString();
        });
        child.on('close', () => {
            if (stdout.trim()) {
                try {
                    resolve(JSON.parse(stdout.trim()));
                    return;
                } catch (err) {
                    resolve({ error: `invalid response: ${stdout.trim()}`, stderr });
                    return;
                }
            }
            if (stderr.trim()) {
                resolve({ error: stderr.trim() });
                return;
            }
            resolve({ action: 'none' });
        });
    });
}

async function promptUser(result) {
    if (!result || result.action === 'none' || result.action === 'suppressed') {
        return;
    }
    const title = result.action === 'sync' ? 'Proofboard project needs sync' : 'Proofboard new project detected';
    const primary = result.action === 'sync' ? 'Sync' : 'Link';
    const secondary = 'Dismiss';
    const choice = await vscode.window.showInformationMessage(
        result.reason || title,
        primary,
        secondary
    );
    if (choice === primary) {
        const terminal = vscode.window.createTerminal('Proofboard');
        terminal.show(true);
        terminal.sendText(result.suggestedCommand || (result.action === 'sync' ? 'proofboard sync' : 'proofboard link'));
    }
}

async function announceOpenWorkspaces() {
    const folders = vscode.workspace.workspaceFolders || [];
    for (const folder of folders) {
        const result = await announceWorkspace(folder.uri.fsPath, 'vscode');
        await promptUser(result);
    }
}

function activate(context) {
    const disposable = vscode.commands.registerCommand('proofboard.detectWorkspace', announceOpenWorkspaces);
    context.subscriptions.push(disposable);
    void announceOpenWorkspaces();
}

function deactivate() {}

module.exports = {
    activate,
    deactivate,
};
