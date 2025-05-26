import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler, LabelEncoder
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import classification_report, accuracy_score
from imblearn.over_sampling import SMOTE
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
import kagglehub
import os

# Download latest version
path = kagglehub.dataset_download("chethuhn/network-intrusion-dataset")

filenames = [
    'Thursday-WorkingHours-Afternoon-Infilteration.pcap_ISCX.csv',
    # 'Monday-WorkingHours.pcap_ISCX.csv',
    # 'Friday-WorkingHours-Morning.pcap_ISCX.csv',
    # 'Friday-WorkingHours-Afternoon-PortScan.pcap_ISCX.csv',
    # 'Friday-WorkingHours-Afternoon-DDos.pcap_ISCX.csv',
    # 'Tuesday-WorkingHours.pcap_ISCX.csv',
    # 'Wednesday-workingHours.pcap_ISCX.csv',
    # 'Thursday-WorkingHours-Morning-WebAttacks.pcap_ISCX.csv',
]

dfs = []
for file in filenames:
    file_path = os.path.join(path, file)
    print(f"Загружается: {file}")
    try:
        df_part = pd.read_csv(file_path, low_memory=False)
        dfs.append(df_part)
    except Exception as e:
        print(f"Ошибка при загрузке {file}: {e}")

# 1. Загрузка данных CICIDS2017
df = pd.concat(dfs, ignore_index=True)

print(f"\nОбъединено строк: {len(df)}")
print("Пример данных:")
print(df.head())

df.columns = df.columns.str.strip()

# Проверка наличия нужного столбца
if 'Label' not in df.columns:
    print("Столбец 'Label' не найден. Доступные столбцы:")
    print(df.columns.tolist())
    raise ValueError("В одном или нескольких файлах отсутствует колонка 'Label'")

# 2. Предобработка данных
# Удаляем неинформативные или дублирующие колонки, например 'Flow ID', 'Timestamp' и др.
df = df.drop(['Flow ID', 'Source IP', 'Destination IP', 'Timestamp'], axis=1, errors='ignore')

# Обработка меток: бинаризация (Benign -> 0, все атаки -> 1)
df['Label'] = df['Label'].apply(lambda x: 0 if x == 'BENIGN' else 1)

# Отделяем признаки и целевой класс
X = df.drop('Label', axis=1)
y = df['Label']

# Заменим бесконечности и слишком большие значения на NaN
X.replace([np.inf, -np.inf], np.nan, inplace=True)

# Проверим максимально допустимое значение для float64 (в теории около 1.8e308)
# но лучше ограничить разумным значением (например, 1e10)
X = X.applymap(lambda x: np.nan if isinstance(x, (int, float)) and abs(x) > 1e10 else x)

# Обработка пропусков (если есть)
X.fillna(0, inplace=True)

print("Входной вектор (features):")
print(X.columns.tolist())
print("Размерность входного вектора:", X.shape[1])

# 3. Масштабирование признаков
scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)

# 1. Разделение на обучающую и тестовую выборки
X_train, X_test, y_train, y_test = train_test_split(
    X_scaled, y, test_size=0.2, random_state=42, stratify=y)

# 2. Балансировка классов на тренировочных данных
smote = SMOTE(random_state=42)
X_train_resampled, y_train_resampled = smote.fit_resample(X_train, y_train)

# 3. Обучение модели
model = RandomForestClassifier(n_estimators=150, max_depth=20, random_state=42, n_jobs=-1)
model.fit(X_train_resampled, y_train_resampled)

# 4. Оценка модели
y_pred = model.predict(X_test)
print(classification_report(y_test, y_pred))

# 9. Конвертация модели в ONNX
initial_type = [('float_input', FloatTensorType([None, X.shape[1]]))]
onnx_model = convert_sklearn(model, initial_types=initial_type, target_opset=12)

print("ONNX inputs:")
for inp in onnx_model.graph.input:
    print(f"- {inp.name}")

print("ONNX outputs:")
for out in onnx_model.graph.output:
    print(f"- {out.name}")
    
with open('rf_cicids2017.onnx', 'wb') as f:
    f.write(onnx_model.SerializeToString())

print('ONNX модель успешно сохранена как rf_cicids2017.onnx')
