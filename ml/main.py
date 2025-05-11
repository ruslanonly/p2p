import pandas as pd
from sklearn.tree import DecisionTreeClassifier
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import LabelEncoder
from sklearn.metrics import classification_report
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
import json

df = pd.read_csv("./datasets/kddcup.data.corrected", header=None)

column_names = [
    "duration","protocol_type", "service", "flag", "src_bytes",
    "dst_bytes", "land", "wrong_fragment", "urgent", "hot",
    "num_failed_logins", "logged_in", "num_compromised", "root_shell", "su_attempted",
    "num_root", "num_file_creations", "num_shells", "num_access_files", "num_outbound_cmds",
    "is_host_login", "is_guest_login", "count", "srv_count", "serror_rate",
    "srv_serror_rate", "rerror_rate", "srv_rerror_rate","same_srv_rate", "diff_srv_rate",
    "srv_diff_host_rate", "dst_host_count", "dst_host_srv_count", "dst_host_same_srv_rate", "dst_host_diff_srv_rate",
    "dst_host_same_src_port_rate", "dst_host_srv_diff_host_rate", "dst_host_serror_rate","dst_host_srv_serror_rate", "dst_host_rerror_rate",
    "dst_host_srv_rerror_rate", "label"
]

df.columns = column_names

df["label"] = df["label"].apply(lambda x: "green" if x == "normal." else "red")

category_columns = ["protocol_type", "service", "flag"]
for col in category_columns:
    le = LabelEncoder()
    df[col] = le.fit_transform(df[col])
    mapping = {str(k): int(v) for k, v in zip(le.classes_, le.transform(le.classes_))}

    with open(f"{col}_mapping.json", "w") as f:
        json.dump(mapping, f, indent=2)

y = df["label"]
X = df.drop(columns=["label"])

le_y = LabelEncoder()
y_encoded = le_y.fit_transform(y)

X_train, X_test, y_train, y_test = train_test_split(X, y_encoded, test_size=0.2, random_state=42)

clf = DecisionTreeClassifier(max_depth=5)
clf.fit(X_train, y_train)

print("Классы:", le_y.classes_)

initial_type = [("input", FloatTensorType([None, X.shape[1]]))]  # X.shape[1] = 41
onnx_model = convert_sklearn(clf, initial_types=initial_type, options={id(clf): {"zipmap": False}})

with open("model.onnx", "wb") as f:
    f.write(onnx_model.SerializeToString())