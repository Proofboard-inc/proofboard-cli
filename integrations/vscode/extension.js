const cp = require('child_process');
const vscode = require('vscode');

function cliPath() {
    return process.env.PROOFBOARD_CLI_PATH || 'proofboard';
}

function runDetect(workspacePath, editorName) {
    return new Promise((resolve) => {
        const args = ['detect', '--workspace', workspacePath, '--editor', editorName, '--json'];
        cp.execFile(cliPath(), args, {
            env: process.env,
            cwd: workspacePath,
            maxBuffer: 1024 * 1024,
        }, (error, stdout) => {
            if (error && !stdout) {
                resolve(null);
                return;
            }

            const text = String(stdout || '').trim();
            if (!text) {
                resolve(null);
                return;
            }

            try {
                resolve(JSON.parse(text));
            } catch (_err) {
                resolve(null);
            }
        });
    });
}

function spawnSyncForWorkspace(workspacePath) {
    const child = cp.spawn(cliPath(), ['sync'], {
        env: process.env,
        cwd: workspacePath,
        stdio: 'ignore',
        detached: true,
    });
    child.unref();
}

async function showDetectionNotification(workspacePath, result) {
    if (!result || !result.action || result.action === 'none' || result.action === 'suppressed') {
        return;
    }

    const title = result.action === 'sync' ? 'Project needs sync' : 'New project detected';
    const body = result.action === 'sync'
        ? `Run Proofboard sync to capture the latest work.\n${result.repoName || workspacePath}`
        : `Add this workspace to Proofboard.\n${result.repoName || workspacePath}`;
    const primary = result.action === 'sync' ? 'Sync now' : 'Add to Proofboard';
    const secondary = result.action === 'sync' ? 'Later' : 'Not this project';

    const selection = await vscode.window.showInformationMessage(
        `${title}\n${body}`,
        { modal: false },
        primary,
        secondary,
    );

    if (selection === primary) {
        spawnSyncForWorkspace(workspacePath);
    }
}

async function announceOpenWorkspaces() {
    const folders = vscode.workspace.workspaceFolders || [];
    await Promise.all(folders.map(async (folder) => {
        const result = await runDetect(folder.uri.fsPath, 'vscode');
        await showDetectionNotification(folder.uri.fsPath, result);
    }));
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
